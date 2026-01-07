package print_handler

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"print-pro-backend/internal/middleware/auth_middleware"
	"strconv"
)

// EditPrintJobOptionsRequest represents the request to edit print job options
type EditPrintJobOptionsRequest struct {
	Filename   string `json:"filename" binding:"required"`
	Color      *bool   `json:"color,omitempty"`      // Optional: true for color, false for B&W
	NumCopies  *int    `json:"num_copies,omitempty"` // Optional: number of copies
	StartPage  *int    `json:"start_page,omitempty"` // Optional: starting page (1-indexed)
	EndPage    *int    `json:"end_page,omitempty"`   // Optional: ending page (1-indexed)
	PageFilterType *string `json:"page_filter_type,omitempty"` // Optional: "all" (default), "odd", "even"
	IndividualColorPrintPages []int `json:"individual_color_print_pages,omitempty"` // Optional: Array of page numbers (1-indexed) to print in color
	SkipPages []int `json:"skip_pages,omitempty"` // Optional: Array of page numbers (1-indexed) to skip during printing
	BackToBack *bool `json:"back_to_back,omitempty"` // Optional: true for duplex printing (both sides), false for simplex (one side)
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
	}

	// Validate page range if both are provided
	if req.StartPage != nil && req.EndPage != nil {
		if *req.StartPage > *req.EndPage {
			log.Printf("WARNING: Invalid page range - start_page (%d) > end_page (%d)", *req.StartPage, *req.EndPage)
			h.sendErrorResponse(w, http.StatusBadRequest, "Invalid page range", "start_page must be less than or equal to end_page")
			return
		}
		if *req.StartPage < 1 || *req.EndPage < 1 {
			log.Printf("WARNING: Invalid page range - pages must be >= 1")
			h.sendErrorResponse(w, http.StatusBadRequest, "Invalid page range", "Page numbers must be greater than 0")
			return
		}
	}

	// Validate num_copies if provided
	if req.NumCopies != nil && *req.NumCopies < 1 {
		log.Printf("WARNING: Invalid num_copies - must be >= 1")
		h.sendErrorResponse(w, http.StatusBadRequest, "Invalid num_copies", "Number of copies must be greater than 0")
		return
	}

	// Validate page_filter_type if provided
	if req.PageFilterType != nil {
		switch *req.PageFilterType {
		case "all", "odd", "even":
		default:
			log.Printf("WARNING: Invalid page_filter_type '%s'", *req.PageFilterType)
			h.sendErrorResponse(w, http.StatusBadRequest, "Invalid page_filter_type", "Allowed values: all, odd, even")
			return
		}
	}

	// Validate skip_pages if provided
	if req.SkipPages != nil {
		for _, pageNum := range req.SkipPages {
			if pageNum < 1 {
				log.Printf("WARNING: Invalid skip_pages - page numbers must be >= 1, found: %d", pageNum)
				h.sendErrorResponse(w, http.StatusBadRequest, "Invalid skip_pages", "Page numbers in skip_pages must be greater than 0")
				return
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
	// Handle individual_color_print_pages: if provided, update it
	// Empty array [] means clear individual color pages (set to NULL, use global color setting)
	// nil means don't update (keep existing value)
	// We use a pointer to distinguish "not provided" (nil) from "empty array provided" (pointer to empty slice)
	var individualColorPagesPtr *[]int
	if req.IndividualColorPrintPages != nil {
		// Field was provided in request (could be empty or non-empty)
		// Always set the pointer so repository knows to update
		// Empty slice will be handled in repository to set NULL
		individualColorPagesPtr = &req.IndividualColorPrintPages
	}
	// If req.IndividualColorPrintPages is nil, individualColorPagesPtr stays nil (don't update)
	
	// Handle skip_pages: if provided, update it
	// Empty array [] means clear skip pages (set to NULL, no pages to skip)
	// nil means don't update (keep existing value)
	var skipPagesPtr *[]int
	if req.SkipPages != nil {
		// Field was provided in request (could be empty or non-empty)
		// Always set the pointer so repository knows to update
		// Empty slice will be handled in repository to set NULL
		skipPagesPtr = &req.SkipPages
	}
	// If req.SkipPages is nil, skipPagesPtr stays nil (don't update)
	
	err = h.printJobRepo.UpdatePrintOptions(ctx, req.Filename, partnerID, accountIDPtr, req.Color, req.NumCopies, req.StartPage, req.EndPage, req.PageFilterType, individualColorPagesPtr, skipPagesPtr, req.BackToBack)
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

	log.Printf("SUCCESS: Print job options updated - User: %s (Type: %s), Filename: %s, Color: %v, NumCopies: %v, StartPage: %v, EndPage: %v, PageFilterType: %v, IndividualColorPages: %v, SkipPages: %v, BackToBack: %v",
		user.ID, user.UserType, req.Filename, req.Color, req.NumCopies, req.StartPage, req.EndPage, req.PageFilterType, req.IndividualColorPrintPages, req.SkipPages, req.BackToBack)

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
		response["start_page"] = updatedJob.StartPage
		response["end_page"] = updatedJob.EndPage
		response["page_filter_type"] = updatedJob.PageFilterType
		response["individual_color_print_pages"] = updatedJob.IndividualColorPrintPages
		response["skip_pages"] = updatedJob.SkipPages
		response["back_to_back"] = updatedJob.BackToBack
	} else {
		// Fallback to request values if we couldn't retrieve updated job
		response["color"] = req.Color
		response["num_copies"] = req.NumCopies
		response["start_page"] = req.StartPage
		response["end_page"] = req.EndPage
		response["page_filter_type"] = req.PageFilterType
		response["individual_color_print_pages"] = req.IndividualColorPrintPages
		response["skip_pages"] = req.SkipPages
		response["back_to_back"] = req.BackToBack
	}

	h.sendJSONResponse(w, http.StatusOK, response)
}

