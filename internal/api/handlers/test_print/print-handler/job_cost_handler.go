package print_handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"
	"print-pro-backend/internal/middleware/auth_middleware"
	"print-pro-backend/internal/models/jobcost"
)

// toJobCostDTO converts a database JobCost model to a JobCostDTO for API response
// This function hides the internal database schema structure
// username is fetched from accounts table using customer_email
func toJobCostDTO(cost *jobcost.JobCost, username *string) JobCostDTO {
	// Format timestamps
	createdAt := ""
	if cost.CreatedAt != nil {
		createdAt = cost.CreatedAt.Format(time.RFC3339)
	}
	
	updatedAt := ""
	if cost.UpdatedAt != nil {
		updatedAt = cost.UpdatedAt.Format(time.RFC3339)
	}

	return JobCostDTO{
		JobID:      cost.PrintJobID,      // Renamed from print_job_id
		Username:   username,             // Username from accounts table
		PageInfo: PageInfoDTO{
			TotalPages:      cost.TotalPages,
			PagesToPrint:    cost.PagesToPrint,
			ColorPages:      cost.ColorPages,
			BlackWhitePages: cost.BlackWhitePages,
		},
		Pricing: PricingDTO{
			NumCopies: cost.NumCopies,
			TotalCost: cost.TotalCost,    // Renamed from total_cost
			Currency:  "INR",             // Default currency
		},
		Timestamp: TimestampDTO{
			CreatedAt: createdAt,
			UpdatedAt: updatedAt,
		},
	}
}

// GetJobCosts handles GET /api/test-print/job-costs
// Returns all job costs from the job_cost table (no filtering)
func (h *PrintHandler) GetJobCosts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.sendErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed", "Only GET method is allowed")
		return
	}

	// Get authenticated user from context
	user, ok := auth_middleware.GetUserFromContext(r)
	if !ok {
		h.sendErrorResponse(w, http.StatusUnauthorized, "Unauthorized", "User not found in session")
		return
	}

	ctx := r.Context()

	// Parse query parameters for month/year filtering (optional)
	yearParam := r.URL.Query().Get("year")
	monthParam := r.URL.Query().Get("month")

	var costs []jobcost.JobCost
	var err error
	var cacheKey string
	var shouldCache bool

	// If both year and month are provided, filter by month/year with caching
	if yearParam != "" && monthParam != "" {
		year, errYear := strconv.Atoi(yearParam)
		month, errMonth := strconv.Atoi(monthParam)
		
		if errYear != nil || errMonth != nil {
			h.sendErrorResponse(w, http.StatusBadRequest, "Invalid parameters", "Year and month must be valid integers")
			return
		}
		
		// Validate month range (1-12)
		if month < 1 || month > 12 {
			h.sendErrorResponse(w, http.StatusBadRequest, "Invalid month", "Month must be between 1 and 12")
			return
		}
		
		// Validate year (reasonable range, e.g., 2000-2100)
		if year < 2000 || year > 2100 {
			h.sendErrorResponse(w, http.StatusBadRequest, "Invalid year", "Year must be between 2000 and 2100")
			return
		}
		
		// Create cache key for this month/year
		cacheKey = fmt.Sprintf("job_costs:%d:%d", year, month)
		shouldCache = true
		
		// Try to get from cache first
		if h.redisClient != nil {
			cachedData, err := h.redisClient.Get(ctx, cacheKey)
			if err == nil && cachedData != "" {
				// Cache hit - unmarshal and return
				var cachedCosts []jobcost.JobCost
				if err := json.Unmarshal([]byte(cachedData), &cachedCosts); err == nil {
					log.Printf("CACHE HIT: Retrieved %d job costs from cache for %d/%d", len(cachedCosts), month, year)
					costs = cachedCosts
					
					// Also cache previous month in background (for faster access)
					go h.cachePreviousMonth(ctx, year, month)
					
					// Skip database query and go to username fetching
					goto fetchUsernames
				} else {
					log.Printf("WARNING: Failed to unmarshal cached data for %s - %v", cacheKey, err)
				}
			} else {
				log.Printf("CACHE MISS: No cached data found for %s", cacheKey)
			}
		}
		
		// Cache miss or Redis unavailable - fetch from database
		costs, err = h.jobCostRepo.GetByMonth(ctx, year, month)
		if err != nil {
			log.Printf("ERROR: Failed to get job costs for month/year - %v", err)
			h.sendErrorResponse(w, http.StatusInternalServerError, "Failed to retrieve job costs", err.Error())
			return
		}
		
		// Store in cache with 1 minute TTL
		if h.redisClient != nil && shouldCache {
			costsJSON, err := json.Marshal(costs)
			if err == nil {
				if err := h.redisClient.Set(ctx, cacheKey, costsJSON, 1*time.Minute); err == nil {
					log.Printf("CACHE SET: Stored %d job costs in cache for %d/%d (TTL: 1 minute)", len(costs), month, year)
				} else {
					log.Printf("WARNING: Failed to store job costs in cache - %v", err)
				}
			} else {
				log.Printf("WARNING: Failed to marshal job costs for caching - %v", err)
			}
			
			// Also cache previous month in background (for faster access)
			go h.cachePreviousMonth(ctx, year, month)
		}
		
		log.Printf("SUCCESS: Retrieved %d job costs for %d/%d", len(costs), month, year)
	} else {
		// Fetch all job costs from the table (no filtering, no caching)
		// Pass nil to GetAll to get all records
		costs, err = h.jobCostRepo.GetAll(ctx, nil)
		if err != nil {
			log.Printf("ERROR: Failed to get job costs - %v", err)
			h.sendErrorResponse(w, http.StatusInternalServerError, "Failed to retrieve job costs", err.Error())
			return
		}
		log.Printf("SUCCESS: Retrieved %d job costs for user (type: %s)", len(costs), user.UserType)
	}

fetchUsernames:

	// Collect unique customer emails and fetch usernames
	customerEmailSet := make(map[string]bool)
	for _, cost := range costs {
		if cost.AccountEmail != "" {
			customerEmailSet[cost.AccountEmail] = true
		}
	}

	// Count unique users (unique customer emails)
	totalUsers := len(customerEmailSet)

	// Fetch usernames for all customer emails
	customerUsernameMap := make(map[string]*string) // customer_email -> username
	for customerEmail := range customerEmailSet {
		account, err := h.accountRepository.GetByEmail(ctx, customerEmail)
		if err != nil {
			log.Printf("WARNING: Failed to fetch username for customer_email %s - %v", customerEmail, err)
			customerUsernameMap[customerEmail] = nil // Set to nil if not found
		} else {
			customerUsernameMap[customerEmail] = account.Username
		}
	}

	// Convert database models to DTOs with usernames
	jobCostDTOs := make([]JobCostDTO, len(costs))
	for i := range costs {
		username := customerUsernameMap[costs[i].AccountEmail]
		jobCostDTOs[i] = toJobCostDTO(&costs[i], username)
	}

	// Return response using DTO structure
	response := JobCostsResponse{
		Success:    true,
		Message:    "Job costs retrieved successfully",
		Data:       jobCostDTOs,
		Count:      len(jobCostDTOs),
		TotalUsers: totalUsers, // Count of unique customer emails
	}

	h.sendJSONResponse(w, http.StatusOK, response)
}

// cachePreviousMonth caches the previous month's job costs in the background
// This helps reduce database reads when users navigate between months
func (h *PrintHandler) cachePreviousMonth(ctx context.Context, year, month int) {
	if h.redisClient == nil {
		return
	}
	
	// Calculate previous month
	prevYear := year
	prevMonth := month - 1
	if prevMonth < 1 {
		prevMonth = 12
		prevYear = year - 1
	}
	
	// Check if already cached
	prevCacheKey := fmt.Sprintf("job_costs:%d:%d", prevYear, prevMonth)
	exists, err := h.redisClient.Exists(ctx, prevCacheKey)
	if err == nil && exists {
		log.Printf("CACHE: Previous month %d/%d already cached, skipping", prevMonth, prevYear)
		return
	}
	
	// Fetch previous month's data
	prevCosts, err := h.jobCostRepo.GetByMonth(ctx, prevYear, prevMonth)
	if err != nil {
		log.Printf("WARNING: Failed to fetch previous month's job costs for caching - %v", err)
		return
	}
	
	// Store in cache with 1 minute TTL
	prevCostsJSON, err := json.Marshal(prevCosts)
	if err == nil {
		if err := h.redisClient.Set(ctx, prevCacheKey, prevCostsJSON, 1*time.Minute); err == nil {
			log.Printf("CACHE: Pre-cached %d job costs for previous month %d/%d (TTL: 1 minute)", len(prevCosts), prevMonth, prevYear)
		} else {
			log.Printf("WARNING: Failed to cache previous month's job costs - %v", err)
		}
	} else {
		log.Printf("WARNING: Failed to marshal previous month's job costs for caching - %v", err)
	}
}
