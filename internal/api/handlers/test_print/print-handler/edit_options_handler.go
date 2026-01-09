package print_handler

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"print-pro-backend/internal/middleware/auth_middleware"
	"print-pro-backend/internal/models/printjob"
	"strconv"
)

// EditPrintJobOptionsRequest represents the request to edit print job options
type EditPrintJobOptionsRequest struct {
	Filename   string `json:"filename" binding:"required"`
	Color      *bool   `json:"color,omitempty"`      // Optional: true for color, false for B&W
	NumCopies  *int    `json:"num_copies,omitempty"` // Optional: number of copies
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

	// Get account_id from user.ID
	accountID, err := strconv.ParseInt(user.ID, 10, 64)
	if err != nil {
		log.Printf("ERROR: Failed to parse user ID '%s' - %v", user.ID, err)
		h.sendErrorResponse(w, http.StatusBadRequest, "Invalid user ID", "Invalid account ID format")
		return
	}

	// Get the print job from database to verify ownership
	printJob, err := h.printJobRepo.GetByFilename(ctx, req.Filename)
	if err != nil {
		log.Printf("ERROR: Print job not found for filename '%s' - %v", req.Filename, err)
		h.sendErrorResponse(w, http.StatusNotFound, "File not found", fmt.Sprintf("Print job not found for filename: %s", req.Filename))
		return
	}

	// SECURITY: Verify ownership based on user type
	var partnerID int64
	if user.UserType == "partner" {
		// For partners: verify file belongs to their shop
		partnerProfile, err := h.partnerProfileRepo.GetByAccountID(ctx, accountID)
		if err != nil {
			log.Printf("ERROR: Partner profile not found for account_id: %d - %v", accountID, err)
			h.sendErrorResponse(w, http.StatusNotFound, "Partner profile not found", "Partner profile not found for this account")
			return
		}

		partnerID = partnerProfile.ID
		log.Printf("INFO: Edit request - Partner: %s (partner_id: %d), Filename: %s", user.ID, partnerID, req.Filename)

		// SECURITY: Verify the print job belongs to this partner's shop
		if printJob.PartnerID != partnerID {
			log.Printf("ERROR: SECURITY - Partner %d trying to edit file belonging to partner %d - Filename: %s", 
				partnerID, printJob.PartnerID, req.Filename)
			h.sendErrorResponse(w, http.StatusForbidden, "Access denied", "You do not have permission to edit this file")
			return
		}
	} else if user.UserType == "customer" {
		// For customers: verify file belongs to them
		log.Printf("INFO: Edit request - Customer: %s (account_id: %d), Filename: %s", user.ID, accountID, req.Filename)

		// SECURITY: Verify the print job belongs to this customer
		if printJob.AccountID == nil || *printJob.AccountID != accountID {
			log.Printf("ERROR: SECURITY - Customer %d trying to edit file belonging to account %v - Filename: %s", 
				accountID, printJob.AccountID, req.Filename)
			h.sendErrorResponse(w, http.StatusForbidden, "Access denied", "You do not have permission to edit this file")
			return
		}

		// Use the file's partner_id for the update (customer's file belongs to a specific shop)
		partnerID = printJob.PartnerID
		
		// SECURITY: Customers cannot set delete_after_print - only partners can control this
		if req.DeleteAfterPrint != nil {
			log.Printf("WARNING: Customer %d attempted to set delete_after_print - ignoring this field", accountID)
			req.DeleteAfterPrint = nil // Ignore customer's attempt to set this
		}
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
	// For customers, pass accountID for additional security check
	var accountIDPtr *int64
	if user.UserType == "customer" {
		accountIDPtr = &accountID
	}
	
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
	
	err = h.printJobRepo.UpdatePrintOptions(ctx, req.Filename, partnerID, accountIDPtr, req.Color, req.NumCopies, startPagePtr, endPagePtr, pageFilterTypePtr, individualColorPagesPtr, skipPagesPtr, req.BackToBack, req.DeleteAfterPrint)
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

