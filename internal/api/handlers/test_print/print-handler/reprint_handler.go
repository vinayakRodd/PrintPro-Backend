package print_handler

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"print-pro-backend/internal/middleware/auth_middleware"
	"print-pro-backend/internal/models/printjob"
)

// ReprintJob handles reprint requests - re-inserts a completed job with status "pending"
func (h *PrintHandler) ReprintJob(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.sendErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed", "Only POST method is allowed")
		return
	}

	// Get user from context
	user, ok := auth_middleware.GetUserFromContext(r)
	if !ok {
		h.sendErrorResponse(w, http.StatusUnauthorized, "Unauthorized", "User not found in session")
		return
	}

	// Verify user is a partner
	if user.UserType != "partner" {
		h.sendErrorResponse(w, http.StatusForbidden, "Forbidden", "Only partners can reprint jobs")
		return
	}

	ctx := r.Context()

	// Get partner_email from account_email (user.ID is the email)
	accountEmail := user.ID

	// Get partner profile to get partner_email
	partnerProfile, err := h.partnerProfileRepo.GetByAccountEmail(ctx, accountEmail)
	if err != nil {
		log.Printf("ERROR: Partner profile not found - %v", err)
		h.sendErrorResponse(w, http.StatusNotFound, "Partner profile not found", "Partner profile not found for this account")
		return
	}

	partnerEmail := partnerProfile.PartnerEmail
	log.Printf("INFO: Reprint request from partner (shop: %s)", partnerProfile.ShopName)

	// Parse request body
	var req struct {
		Filename string `json:"filename"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("ERROR: Failed to decode request body - %v", err)
		h.sendErrorResponse(w, http.StatusBadRequest, "Invalid request body", "Failed to parse JSON: "+err.Error())
		return
	}

	// Validate filename
	if req.Filename == "" {
		log.Printf("ERROR: Empty filename in reprint request")
		h.sendErrorResponse(w, http.StatusBadRequest, "Filename required", "Please provide a filename")
		return
	}

	// Get the existing completed job by filename
	existingJob, err := h.printJobRepo.GetByFilename(ctx, req.Filename)
	if err != nil {
		log.Printf("ERROR: Failed to get print job by filename '%s' - %v", req.Filename, err)
		h.sendErrorResponse(w, http.StatusNotFound, "Job not found", fmt.Sprintf("Print job with filename '%s' not found", req.Filename))
		return
	}

	// SECURITY: Verify the job belongs to this partner
	if existingJob.PartnerEmail == nil || *existingJob.PartnerEmail != partnerEmail {
		log.Printf("ERROR: SECURITY ISSUE - Partner '%s' attempted to reprint job belonging to different partner", partnerEmail)
		h.sendErrorResponse(w, http.StatusForbidden, "Forbidden", "You do not have permission to reprint this job")
		return
	}

	// Verify the job is completed (should be, but double-check)
	if existingJob.Status != nil && *existingJob.Status != printjob.StatusCompleted {
		log.Printf("WARNING: Attempted to reprint job with status '%s' (expected 'completed')", *existingJob.Status)
		// Allow reprint anyway - might be useful to reprint pending/failed jobs
	}

	// Extract all job data for reprint
	var accountEmailPtr *string
	if existingJob.CustomerEmail != nil {
		accountEmailPtr = existingJob.CustomerEmail
	}

	// Extract page options
	var startPage, endPage *int
	var pageFilterType *string
	var individualColorPages, skipPages []int
	// PageOptions is a struct (not a pointer), so we check if it has any meaningful data
	pageOpts := existingJob.PageOptions
	startPage = pageOpts.StartPage
	endPage = pageOpts.EndPage
	pageFilterType = pageOpts.FilterType
	individualColorPages = pageOpts.ColorPages
	skipPages = pageOpts.SkipPages

	// Create new print job with same data but status "pending"
	newJob, err := h.printJobRepo.Create(
		ctx,
		accountEmailPtr,
		partnerEmail,
		existingJob.Filename,
		existingJob.FileURL,
		existingJob.PType,
		existingJob.Color,
		existingJob.NumCopies,
		startPage,
		endPage,
		pageFilterType,
		individualColorPages,
		skipPages,
		existingJob.BackToBack,
		existingJob.DeleteAfterPrint,
	)
	if err != nil {
		log.Printf("ERROR: Failed to create reprint job - %v", err)
		h.sendErrorResponse(w, http.StatusInternalServerError, "Internal error", "Failed to create reprint job: "+err.Error())
		return
	}

	// SECURITY: Verify the created print job has the correct partner_email
	if newJob.PartnerEmail == nil || *newJob.PartnerEmail != partnerEmail {
		log.Printf("ERROR: SECURITY ISSUE - Reprint job created with wrong partner_email - File: %s", req.Filename)
		h.sendErrorResponse(w, http.StatusInternalServerError, "Internal error", "Security validation failed")
		return
	}

	log.Printf("SUCCESS: Reprint job created - ID: %d, Shop: %s, File: %s, Status: %s",
		newJob.ID, partnerProfile.ShopName, req.Filename, *newJob.Status)

	h.sendJSONResponse(w, http.StatusOK, map[string]interface{}{
		"success":  true,
		"message":  "Job reprinted successfully - moved to print queue with status 'pending'",
		"job_id":   newJob.ID,
		"filename": newJob.Filename,
		"status":   newJob.Status,
	})
}
