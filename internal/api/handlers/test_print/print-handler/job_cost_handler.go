package print_handler

import (
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

	// If both year and month are provided, filter by month/year
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
		
		// Fetch job costs for the specified month/year
		costs, err = h.jobCostRepo.GetByMonth(ctx, year, month)
		if err != nil {
			log.Printf("ERROR: Failed to get job costs for month/year - %v", err)
			h.sendErrorResponse(w, http.StatusInternalServerError, "Failed to retrieve job costs", err.Error())
			return
		}
		log.Printf("SUCCESS: Retrieved %d job costs for %d/%d", len(costs), month, year)
	} else {
		// Fetch all job costs from the table (no filtering)
		// Pass nil to GetAll to get all records
		costs, err = h.jobCostRepo.GetAll(ctx, nil)
		if err != nil {
			log.Printf("ERROR: Failed to get job costs - %v", err)
			h.sendErrorResponse(w, http.StatusInternalServerError, "Failed to retrieve job costs", err.Error())
			return
		}
		log.Printf("SUCCESS: Retrieved %d job costs for user (type: %s)", len(costs), user.UserType)
	}

	log.Printf("SUCCESS: Retrieved %d job costs for user (type: %s)", len(costs), user.UserType)

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
