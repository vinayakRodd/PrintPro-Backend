package print_handler

import (
	"log"
	"net/http"
	"time"
	"print-pro-backend/internal/middleware/auth_middleware"
	"print-pro-backend/internal/models/jobcost"
)

// toJobCostDTO converts a database JobCost model to a JobCostDTO for API response
// This function hides the internal database schema structure
func toJobCostDTO(cost *jobcost.JobCost) JobCostDTO {
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
		CustomerID: cost.AccountEmail,   // Renamed from customer_email
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

	// Fetch all job costs from the table (no filtering)
	// Pass nil to GetAll to get all records
	costs, err := h.jobCostRepo.GetAll(ctx, nil)
	if err != nil {
		log.Printf("ERROR: Failed to get job costs - %v", err)
		h.sendErrorResponse(w, http.StatusInternalServerError, "Failed to retrieve job costs", err.Error())
		return
	}

	log.Printf("SUCCESS: Retrieved %d job costs for user (type: %s)", len(costs), user.UserType)

	// Convert database models to DTOs
	jobCostDTOs := make([]JobCostDTO, len(costs))
	for i := range costs {
		jobCostDTOs[i] = toJobCostDTO(&costs[i])
	}

	// Return response using DTO structure
	response := JobCostsResponse{
		Success: true,
		Message: "Job costs retrieved successfully",
		Data:    jobCostDTOs,
		Count:   len(jobCostDTOs),
	}

	h.sendJSONResponse(w, http.StatusOK, response)
}
