package print_handler

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"print-pro-backend/internal/middleware/auth_middleware"
	"print-pro-backend/internal/models/printjob"
)

// EditPrintJobOptionsRequest represents the request to edit print job options
type EditPrintJobOptionsRequest struct {
	Filename   string `json:"filename" binding:"required"`
	Color      *bool   `json:"color,omitempty"`      // Optional: true for color, false for B&W
	NumCopies  *int    `json:"num_copies,omitempty"` // Optional: number of copies
	PrintType  *string `json:"print_type,omitempty"` // Optional: paper size (e.g., "A4", "Letter")
	CropOptions *printjob.CropOptions `json:"crop_options,omitempty"` // Optional: crop margins in millimeters
	BackToBack *bool `json:"back_to_back,omitempty"` // Optional: true for duplex printing (both sides), false for simplex (one side)
	DeleteAfterPrint *bool `json:"delete_after_print,omitempty"` // Optional: if true, file will be hidden from partner listings after printing is completed
	PageOptions *printjob.PageOptions `json:"page_options,omitempty"` // Optional: Consolidated page options structure
}

// EditPrintJobOptions handles requests from partners and customers to edit print job options
// PUT /api/test-print/edit-options
// Allows partners and customers to update: color, num_copies, start_page, end_page
// - Partners can edit files that belong to their shop
// - Customers can edit files that they uploaded
func (h *PrintHandler) EditPrintJobOptions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut && r.Method != http.MethodPost {
		h.sendErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed", "Only PUT or POST method is allowed")
		return
	}

	// Get user from context (set by auth middleware)
	user, ok := auth_middleware.GetUserFromContext(r)
	if !ok {
		h.sendErrorResponse(w, http.StatusUnauthorized, "Unauthorized", "User not found in session")
		return
	}

	// Verify user is either a partner or customer
	if user.UserType != "partner" && user.UserType != "customer" {
		h.sendErrorResponse(w, http.StatusForbidden, "Forbidden", "Only partners and customers can edit print job options")
		return
	}

	// Parse request body
	var req EditPrintJobOptionsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("ERROR: Failed to decode request body - %v", err)
		h.sendErrorResponse(w, http.StatusBadRequest, "Invalid request body", "Failed to parse JSON: "+err.Error())
		return
	}

	// Validate filename
	if req.Filename == "" {
		log.Printf("ERROR: Empty filename in edit request")
		h.sendErrorResponse(w, http.StatusBadRequest, "Filename required", "Please provide a filename")
		return
	}

	ctx := r.Context()

	// Get account_email from user.ID (user.ID is the email)
	accountEmail := user.ID

	// Get the print job from database to verify ownership
	printJob, err := h.printJobRepo.GetByFilename(ctx, req.Filename)
	if err != nil {
		log.Printf("ERROR: Print job not found for filename '%s' - %v", req.Filename, err)
		h.sendErrorResponse(w, http.StatusNotFound, "File not found", fmt.Sprintf("Print job not found for filename: %s", req.Filename))
		return
	}

	// SECURITY: Verify ownership based on user type
	var partnerEmail string
	if user.UserType == "partner" {
		// For partners: verify file belongs to their shop
		partnerProfile, err := h.partnerProfileRepo.GetByAccountEmail(ctx, accountEmail)
		if err != nil {
			log.Printf("ERROR: Partner profile not found - %v", err)
			h.sendErrorResponse(w, http.StatusNotFound, "Partner profile not found", "Partner profile not found for this account")
			return
		}

		partnerEmail = partnerProfile.PartnerEmail
		log.Printf("INFO: Edit request - Partner authorized, Filename: %s", req.Filename)

		// SECURITY: Verify the print job belongs to this partner's shop
		if printJob.PartnerEmail == nil || *printJob.PartnerEmail != partnerEmail {
			log.Printf("ERROR: SECURITY - Partner trying to edit file belonging to different partner - Filename: %s", req.Filename)
			h.sendErrorResponse(w, http.StatusForbidden, "Access denied", "You do not have permission to edit this file")
			return
		}
	} else if user.UserType == "customer" {
		// For customers: verify file belongs to them
		log.Printf("INFO: Edit request - Customer authorized, Filename: %s", req.Filename)

		// SECURITY: Verify the print job belongs to this customer
		if printJob.CustomerEmail == nil || *printJob.CustomerEmail != accountEmail {
			log.Printf("ERROR: SECURITY - Customer trying to edit file belonging to different account - Filename: %s", req.Filename)
			h.sendErrorResponse(w, http.StatusForbidden, "Access denied", "You do not have permission to edit this file")
			return
		}

		// Use the file's partner_email for the update (customer's file belongs to a specific shop)
		if printJob.PartnerEmail == nil {
			log.Printf("ERROR: Print job has no partner_email - Filename: %s", req.Filename)
			h.sendErrorResponse(w, http.StatusInternalServerError, "Internal error", "Print job has no associated shop")
			return
		}
		partnerEmail = *printJob.PartnerEmail
		
		// SECURITY: Customers cannot set delete_after_print - only partners can control this
		if req.DeleteAfterPrint != nil {
			log.Printf("WARNING: Customer attempted to set delete_after_print - ignoring this field")
			req.DeleteAfterPrint = nil // Ignore customer's attempt to set this
		}
	}

	// For customers, pass accountEmail for additional security check
	var accountEmailPtr *string
	if user.UserType == "customer" {
		accountEmailPtr = &accountEmail
	}

	// Validate num_copies if provided
	if req.NumCopies != nil && *req.NumCopies < 1 {
		log.Printf("WARNING: Invalid num_copies - must be >= 1")
		h.sendErrorResponse(w, http.StatusBadRequest, "Invalid num_copies", "Number of copies must be greater than 0")
		return
	}

	// Validate page_options if provided
	if req.PageOptions != nil {
		// Validate page range if both are provided
		if req.PageOptions.StartPage != nil && req.PageOptions.EndPage != nil {
			if *req.PageOptions.StartPage > *req.PageOptions.EndPage {
				log.Printf("WARNING: Invalid page range - start_page (%d) > end_page (%d)", *req.PageOptions.StartPage, *req.PageOptions.EndPage)
				h.sendErrorResponse(w, http.StatusBadRequest, "Invalid page range", "start_page must be less than or equal to end_page")
				return
			}
			if *req.PageOptions.StartPage < 1 || *req.PageOptions.EndPage < 1 {
				log.Printf("WARNING: Invalid page range - pages must be >= 1")
				h.sendErrorResponse(w, http.StatusBadRequest, "Invalid page range", "Page numbers must be greater than 0")
				return
			}
		}

		// Validate filter_type if provided
		if req.PageOptions.FilterType != nil {
			switch *req.PageOptions.FilterType {
			case "all", "odd", "even":
			default:
				log.Printf("WARNING: Invalid filter_type '%s'", *req.PageOptions.FilterType)
				h.sendErrorResponse(w, http.StatusBadRequest, "Invalid filter_type", "Allowed values: all, odd, even")
				return
			}
		}

		// Validate skip_pages if provided
		if req.PageOptions.SkipPages != nil {
			for _, pageNum := range req.PageOptions.SkipPages {
				if pageNum < 1 {
					log.Printf("WARNING: Invalid skip_pages - page numbers must be >= 1, found: %d", pageNum)
					h.sendErrorResponse(w, http.StatusBadRequest, "Invalid skip_pages", "Page numbers in skip_pages must be greater than 0")
					return
				}
			}
		}

		// Validate color_pages if provided
		if req.PageOptions.ColorPages != nil {
			for _, pageNum := range req.PageOptions.ColorPages {
				if pageNum < 1 {
					log.Printf("WARNING: Invalid color_pages - page numbers must be >= 1, found: %d", pageNum)
					h.sendErrorResponse(w, http.StatusBadRequest, "Invalid color_pages", "Page numbers in color_pages must be greater than 0")
					return
				}
			}
		}
	}

	// Update print job options
	// Only update fields that are provided (non-nil)
	// Extract page options from page_options structure
	var startPagePtr *int
	var endPagePtr *int
	var pageFilterTypePtr *string
	var individualColorPagesPtr *[]int
	var skipPagesPtr *[]int
	
	if req.PageOptions != nil {
		startPagePtr = req.PageOptions.StartPage
		endPagePtr = req.PageOptions.EndPage
		pageFilterTypePtr = req.PageOptions.FilterType
		if req.PageOptions.ColorPages != nil {
			individualColorPagesPtr = &req.PageOptions.ColorPages
		}
		if req.PageOptions.SkipPages != nil {
			skipPagesPtr = &req.PageOptions.SkipPages
		}
	}
	
	// Validate and normalize print_type if provided
	var validatedPrintType *string
	if req.PrintType != nil {
		validated := printjob.ValidatePaperSize(*req.PrintType)
		validatedPrintType = &validated
	}
	
	// Validate crop_options if provided
	var validatedCropOptions *printjob.CropOptions
	if req.CropOptions != nil {
		// Get paper size for validation (use validated print_type or existing job's print_type)
		paperSize := printjob.PaperSizeDefault
		if validatedPrintType != nil {
			paperSize = *validatedPrintType
		} else if printJob.PrintType != nil {
			paperSize = *printJob.PrintType
		} else if printJob.PType != nil {
			paperSize = *printJob.PType
		}
		
		validated, err := printjob.ValidateCropOptions(req.CropOptions, paperSize)
		if err != nil {
			log.Printf("ERROR: Invalid crop_options - %v", err)
			h.sendErrorResponse(w, http.StatusBadRequest, "Invalid crop options", err.Error())
			return
		}
		validatedCropOptions = validated
	}
	
	err = h.printJobRepo.UpdatePrintOptions(ctx, req.Filename, partnerEmail, accountEmailPtr, req.Color, req.NumCopies, startPagePtr, endPagePtr, pageFilterTypePtr, individualColorPagesPtr, skipPagesPtr, req.BackToBack, req.DeleteAfterPrint, validatedPrintType, validatedCropOptions)
	if err != nil {
		log.Printf("ERROR: Failed to update print job options for '%s' - %v", req.Filename, err)
		h.sendErrorResponse(w, http.StatusInternalServerError, "Internal error", "Failed to update print job options")
		return
	}

	// Get updated print job to return
	updatedJob, err := h.printJobRepo.GetByFilename(ctx, req.Filename)
	if err != nil {
		log.Printf("WARNING: Failed to retrieve updated print job - %v", err)
		// Still return success since update succeeded
	}

	// NOTE: Cost calculation and storage is deferred until job status becomes "completed"
	// This ensures we only store the final cost after successful printing
	// If the job is already completed, we should recalculate cost
	if updatedJob != nil && updatedJob.Status != nil && *updatedJob.Status == "completed" {
		if h.costCalculator != nil && h.jobCostRepo != nil {
			// Get file path
			readyDir := h.agentHandler.GetReadyDir()
			filePath := filepath.Join(readyDir, req.Filename)
			
			// Use updated job's page options
			pageOpts := updatedJob.PageOptions
			
			// Get individual color pages from page options
			individualColorPages := []int{}
			if len(pageOpts.ColorPages) > 0 {
				individualColorPages = pageOpts.ColorPages
			}
			
			// Calculate cost for completed job
			jobCost, err := h.costCalculator.CalculateCost(
				filePath,
				pageOpts,
				updatedJob.Color,
				updatedJob.NumCopies,
				individualColorPages,
			)
			
			if err != nil {
				log.Printf("WARNING: Failed to recalculate cost for completed print job - %v", err)
			} else {
				// Store cost in job_cost table
				// Note: job_cost uses account_email as PK, so we need the customer's email
				// Also store partner_email to track which partner/shop the customer used
				if updatedJob.CustomerEmail == nil {
					log.Printf("WARNING: Cannot store job cost - print job has no customer_email")
				} else {
					err = h.jobCostRepo.CreateOrUpdate(ctx, *updatedJob.CustomerEmail, updatedJob.ID, updatedJob.PartnerEmail, jobCost)
					if err != nil {
						log.Printf("WARNING: Failed to update job cost - %v", err)
					} else {
						// Update total_cost in print_jobs table
						err = h.jobCostRepo.UpdateTotalCostInPrintJob(ctx, updatedJob.ID, jobCost.TotalCost)
						if err != nil {
							log.Printf("WARNING: Failed to update total_cost in print_jobs - %v", err)
						} else {
							log.Printf("SUCCESS: Cost recalculated for completed job - Total: $%.2f (Color: %d pages, B&W: %d pages, Copies: %d)", 
								jobCost.TotalCost, jobCost.ColorPages, jobCost.BlackWhitePages, jobCost.NumCopies)
						}
					}
				}
			}
		}
	} else {
		log.Printf("INFO: Cost calculation deferred - will be calculated when job status becomes 'completed'")
	}

	// Build response
	response := map[string]interface{}{
		"success":  true,
		"message":  "Print job options updated successfully",
		"filename": req.Filename,
	}

	// Include updated values in response
	if updatedJob != nil {
		response["color"] = updatedJob.Color
		response["num_copies"] = updatedJob.NumCopies
		// Include print_type (prefer PrintType over PType)
		if updatedJob.PrintType != nil {
			response["print_type"] = *updatedJob.PrintType
		} else if updatedJob.PType != nil {
			response["print_type"] = *updatedJob.PType
		}
		// crop_options is now included in page_options
		// Always include page_options (even if empty) - matches list API format
		// Log what we're about to return
		log.Printf("DEBUG: Updated job PageOptions - StartPage: %v, EndPage: %v, FilterType: %v, SkipPages: %v, ColorPages: %v",
			updatedJob.PageOptions.StartPage, updatedJob.PageOptions.EndPage, updatedJob.PageOptions.FilterType,
			updatedJob.PageOptions.SkipPages, updatedJob.PageOptions.ColorPages)
		
		response["page_options"] = updatedJob.PageOptions
		response["back_to_back"] = updatedJob.BackToBack
		response["delete_after_print"] = updatedJob.DeleteAfterPrint
		
		// Log the actual response being sent
		responseJSON, _ := json.MarshalIndent(response, "", "  ")
		log.Printf("SUCCESS: Print job options updated - User: %s (Type: %s), Filename: %s\nResponse: %s",
			user.ID, user.UserType, req.Filename, string(responseJSON))
	} else {
		// Fallback to request values if we couldn't retrieve updated job
		response["color"] = req.Color
		response["num_copies"] = req.NumCopies
		response["back_to_back"] = req.BackToBack
		response["delete_after_print"] = req.DeleteAfterPrint
		if req.PageOptions != nil {
			response["page_options"] = req.PageOptions
		} else {
			// Return empty page_options if not provided
			response["page_options"] = printjob.PageOptions{}
		}
		
		log.Printf("WARNING: Using request values in response (could not retrieve updated job) - User: %s, Filename: %s, PageOptions: %+v",
			user.ID, req.Filename, req.PageOptions)
	}

	h.sendJSONResponse(w, http.StatusOK, response)
}

