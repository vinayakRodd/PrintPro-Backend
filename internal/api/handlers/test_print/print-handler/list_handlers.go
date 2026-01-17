package print_handler

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"print-pro-backend/internal/middleware/auth_middleware"
	"print-pro-backend/internal/models/printjob"
)

// ListFiles lists all PDFs from Redis queues and ready folder (NOT archived)
// - Files in ready folder but NOT in Redis ready queue: status "ready" (show "Print" button)
// - Files in Redis ready queue: status "ready" (show "Print" button)
// - Files in Redis processing queue: status "processing" (show "Processing" status)
// - Archived folder: NOT shown (files already sent to agent and completed)
func (h *PrintHandler) ListFiles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.sendErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed", "Only GET method is allowed")
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
		h.sendErrorResponse(w, http.StatusForbidden, "Forbidden", "Only partners can list files")
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
	log.Printf("INFO: Listing files for partner (shop: %s)", partnerProfile.ShopName)

	// SECURITY: Get all print jobs that belong to THIS partner ONLY from database
	// This ensures partners only see files uploaded to their shop, not other shops
	// Database is the source of truth - we ONLY iterate through files in the database for this partner
	// Using GetByPartnerEmail to get full records and verify partner_email matches
	partnerJobs, err := h.printJobRepo.GetByPartnerEmail(ctx, partnerEmail)
	if err != nil {
		log.Printf("ERROR: Failed to get partner print jobs from database: %v (will show no files for security)", err)
		h.sendErrorResponse(w, http.StatusInternalServerError, "Internal error", "Failed to retrieve files")
		return
	}
	log.Printf("INFO: Found %d print jobs in database for partner (shop-specific filtering applied)", len(partnerJobs))

	// SECURITY: Double-check and filter out any jobs that don't match partner_email
	// This is a defensive check in case the database query somehow returns wrong data
	filteredJobs := []printjob.PrintJob{}
	for _, job := range partnerJobs {
		if job.PartnerEmail != nil && *job.PartnerEmail == partnerEmail {
			filteredJobs = append(filteredJobs, job)
		} else {
			log.Printf("ERROR: SECURITY ISSUE - Print job ID %d has mismatched partner_email - REMOVING FROM LIST", job.ID)
		}
	}
	partnerJobs = filteredJobs
	log.Printf("INFO: After security filtering, %d print jobs remain for partner", len(partnerJobs))

	// If no files found for this partner, return empty list
	if len(partnerJobs) == 0 {
		log.Printf("INFO: No files found for partner")
		h.sendJSONResponse(w, http.StatusOK, map[string]interface{}{
			"success": true,
			"files":   []map[string]interface{}{},
			"count":   0,
			"message": "No files found for this shop",
		})
		return
	}

	readyDir := h.agentHandler.GetReadyDir()

	// Ensure directory exists
	os.MkdirAll(readyDir, 0755)

	fileList := []map[string]interface{}{}
	readyCount := 0
	processingCount := 0

	// Get filenames from Redis queues
	readyQueue, _ := h.agentHandler.GetReadyQueue(ctx)
	processingQueue, _ := h.agentHandler.GetProcessingQueue(ctx)

	// Create maps for quick lookup
	readyQueueMap := make(map[string]bool)
	for _, filename := range readyQueue {
		readyQueueMap[filename] = true
	}
	processingQueueMap := make(map[string]bool)
	for _, filename := range processingQueue {
		processingQueueMap[filename] = true
	}

	// SECURITY: ONLY iterate through files that belong to this partner (from database)
	// Do NOT read all files from filesystem - use database as source of truth
	// Create a map to track which files we've verified belong to this partner
	verifiedFiles := make(map[string]printjob.PrintJob)
	for _, job := range partnerJobs {
		// Double-check: Only include if partner_email matches
		if job.PartnerEmail != nil && *job.PartnerEmail == partnerEmail {
			verifiedFiles[job.Filename] = job
		} else {
			log.Printf("ERROR: SECURITY - Job ID %d has mismatched partner_email - EXCLUDED", job.ID)
		}
	}
	
	log.Printf("INFO: Verified %d files belong to partner (shop: %s)", len(verifiedFiles), partnerProfile.ShopName)
	
	// Collect unique customer emails and fetch usernames
	customerEmailSet := make(map[string]bool)
	for _, job := range verifiedFiles {
		if job.CustomerEmail != nil && *job.CustomerEmail != "" {
			customerEmailSet[*job.CustomerEmail] = true
		}
	}
	
	// Fetch usernames for all customer emails
	customerUsernameMap := make(map[string]*string) // customer_email -> username
	for customerEmail := range customerEmailSet {
		account, err := h.accountRepository.GetByEmail(ctx, customerEmail)
		if err != nil {
			log.Printf("WARNING: Failed to fetch username for customer - %v", err)
			customerUsernameMap[customerEmail] = nil // Set to nil if not found
		} else {
			customerUsernameMap[customerEmail] = account.Username
		}
	}
	
	for filename, job := range verifiedFiles {
		// SECURITY: Final verification - ensure this file belongs to this partner
		if job.PartnerEmail == nil || *job.PartnerEmail != partnerEmail {
			log.Printf("ERROR: CRITICAL SECURITY ISSUE - File '%s' (job ID: %d) has mismatched partner_email - SKIPPING", 
				filename, job.ID)
				continue
			}

		// Verify file actually exists in ready folder
		filePath := filepath.Join(readyDir, filename)
		fileInfo, err := os.Stat(filePath)
		if os.IsNotExist(err) {
			log.Printf("DEBUG: File '%s' exists in database but not in ready folder (may be archived)", filename)
			continue
		}
			if err != nil {
			log.Printf("WARNING: Failed to stat file '%s' - %v", filename, err)
				continue
			}

			// Determine status: Check database status first (source of truth), then Redis queues as fallback
			var status string
			
			// CRITICAL: Check database status first - it's the source of truth
			// Database status is updated when:
			// 1. File is sent to agent → status = "processing"
			// 2. Agent confirms → status = "completed"
			if job.Status != nil && *job.Status != "" {
				status = *job.Status
				// Count based on status
				if status == "processing" {
					processingCount++
				} else if status == "ready" || status == "pending" {
					readyCount++
				}
			} else {
				// Fallback to Redis queues if database status is not set
				if processingQueueMap[filename] {
					// File is in processing queue
					status = "processing"
					processingCount++
				} else if readyQueueMap[filename] {
					// File is in ready queue
					status = "ready"
					readyCount++
				} else {
					// File exists in ready folder but not in any queue (newly uploaded, not queued yet)
					status = "ready"
					readyCount++
				}
			}

			// Get print_type (prefer PrintType over PType)
			var printType *string
			if job.PrintType != nil {
				printType = job.PrintType
			} else if job.PType != nil {
				printType = job.PType
			}
			
			// Get username for this customer
			var customerUsername *string
			if job.CustomerEmail != nil {
				customerUsername = customerUsernameMap[*job.CustomerEmail]
			}
			
			fileList = append(fileList, map[string]interface{}{
			"filename":                    filename,
			"size":                        fileInfo.Size(),
			"modified_at":                 fileInfo.ModTime().Format("2006-01-02T15:04:05Z07:00"),
			"status":                      status,
			"preview_url":                 fmt.Sprintf("/api/test-print/preview?filename=%s", filename),
			"color":                       job.Color,
			"num_copies":                  job.NumCopies,
			"p_type":                      job.PType,
			"print_type":                  printType,
			"crop_options":                job.PageOptions.CropOptions,
			"page_options":                job.PageOptions,
			"back_to_back":                job.BackToBack,
			"delete_after_print":         job.DeleteAfterPrint,
			"created_at":                  job.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			"customer_email":              job.CustomerEmail,
			"customer_username":           customerUsername,
		})
	}

	log.Printf("SUCCESS: Files listed from Redis queues - Partner: %s, Ready: %d, Processing: %d, Total: %d",
		user.ID, readyCount, processingCount, len(fileList))

	h.sendJSONResponse(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"files":   fileList,
		"count":   len(fileList),
		"message": "All files from Redis queues and ready folder",
	})
}

// ListPrinters returns the printer list EXACTLY as sent by the partner agent
// NO local printer detection - only returns what partner agent synced via /api/partner-agent/sync-printers
func (h *PrintHandler) ListPrinters(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.sendErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed", "Only GET method is allowed")
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
		h.sendErrorResponse(w, http.StatusForbidden, "Forbidden", "Only partners can list printers")
		return
	}

	// Get synced printer list from partner agent - EXACTLY as sent, NO local detection
	// This returns ONLY what the partner agent sent via sync-printers endpoint
	log.Printf("DEBUG: ListPrinters - About to call GetSyncedPrinters()")
	log.Printf("DEBUG: ListPrinters - agentHandler pointer: %p", h.agentHandler)
	printers := h.agentHandler.GetSyncedPrinters()
	log.Printf("DEBUG: ListPrinters - GetSyncedPrinters() returned %d printers", len(printers))

	log.Printf("INFO: ListPrinters called - Partner authorized, Printer count from partner agent: %d", len(printers))

	// Print the entire printer list being sent to frontend
	if len(printers) > 0 {
		log.Printf("=========================================")
		log.Printf("PRINTER LIST BEING SENT TO FRONTEND:")
		log.Printf("=========================================")
		for i, printer := range printers {
			log.Printf("Printer #%d:", i+1)
			printerJSON, _ := json.MarshalIndent(printer, "  ", "  ")
			log.Printf("  %s", string(printerJSON))
		}
		log.Printf("=========================================")
		log.Printf("Total printers being sent: %d", len(printers))
		log.Printf("=========================================")
	} else {
		log.Printf("WARNING: No printers found in synced list. Partner agent may not have synced yet.")
		log.Printf("DEBUG: Checking if printers exist in Redis...")
	}

	// Ensure printers is not nil (return empty array if nil)
	if printers == nil {
		printers = []map[string]interface{}{}
		log.Printf("WARNING: Printers list was nil, returning empty array")
	}

	log.Printf("SUCCESS: Sending printer list to frontend - Count: %d", len(printers))

	// Prepare response - return EXACTLY what partner agent sent
	response := map[string]interface{}{
		"success":  true,
		"printers": printers, // EXACTLY as received from partner agent - NO modification
		"count":    len(printers),
		"message":  "Printers from synced partner agent list",
	}

	// Log the EXACT response being sent to frontend
	responseJSON, _ := json.MarshalIndent(response, "", "  ")
	log.Printf("=========================================")
	log.Printf("RESPONSE BEING SENT TO FRONTEND:")
	log.Printf("=========================================")
	log.Printf("%s", string(responseJSON))
	log.Printf("=========================================")
	log.Printf("Total printers in response: %d", len(printers))
	log.Printf("=========================================")

	h.sendJSONResponse(w, http.StatusOK, response)

	log.Printf("SUCCESS: Printer list sent to frontend - Count: %d (EXACTLY as received from partner agent)", len(printers))
}

