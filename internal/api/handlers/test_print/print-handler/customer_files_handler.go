package print_handler

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"print-pro-backend/internal/middleware/auth_middleware"
	"print-pro-backend/internal/models/printjob"
	"strings"
)

// ListCustomerFiles lists all files uploaded by the authenticated customer
// GET /api/test-print/my-files
// Returns files with preview URLs and status information
func (h *PrintHandler) ListCustomerFiles(w http.ResponseWriter, r *http.Request) {
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

	ctx := r.Context()

	// Get account_email from user.ID (user.ID is the email)
	accountEmail := user.ID

	var printJobs []printjob.PrintJob

	// Handle partners: fetch only pending files
	if user.UserType == "partner" {
		// Get partner profile to get partner_email
		partnerProfile, err := h.partnerProfileRepo.GetByAccountEmail(ctx, accountEmail)
		if err != nil {
			log.Printf("ERROR: Partner profile not found - %v", err)
			h.sendErrorResponse(w, http.StatusNotFound, "Partner profile not found", "Partner profile not found for this account")
			return
		}

		partnerEmail := partnerProfile.PartnerEmail
		log.Printf("INFO: Partner requested pending files for shop: %s", partnerProfile.ShopName)

		// Get only pending jobs for this partner
		var err2 error
		printJobs, err2 = h.printJobRepo.GetPendingByPartnerEmail(ctx, partnerEmail)
		if err2 != nil {
			log.Printf("ERROR: Failed to get pending print jobs - %v", err2)
			h.sendErrorResponse(w, http.StatusInternalServerError, "Internal error", "Failed to retrieve files")
			return
		}
		log.Printf("INFO: Found %d pending files for partner", len(printJobs))
	} else if user.UserType == "customer" {
		// Handle customers: existing logic with shop_name parameter
		// Get shop_name query parameter (REQUIRED)
		// SECURITY: shop_name is mandatory - customers can only view files for a specific shop
		shopName := r.URL.Query().Get("shop_name")
		if shopName == "" {
			log.Printf("ERROR: shop_name parameter is required but not provided")
			h.sendErrorResponse(w, http.StatusBadRequest, "Shop name required", "Please provide shop_name query parameter to view files for a specific shop")
			return
		}

		shopName = strings.TrimSpace(shopName)
		log.Printf("INFO: Customer requested files for shop: %s", shopName)

		// Get partner_email from shop_name
		partnerProfile, err := h.partnerProfileRepo.GetByShopName(ctx, shopName)
		if err != nil {
			log.Printf("ERROR: Shop not found: %s - %v", shopName, err)
			h.sendErrorResponse(w, http.StatusBadRequest, "Invalid shop", fmt.Sprintf("Shop '%s' not found", shopName))
			return
		}

		partnerEmail := partnerProfile.PartnerEmail
		log.Printf("INFO: Querying files for customer AND shop '%s'", shopName)

		// SECURITY: Query database with BOTH account_email AND partner_email - database-level filtering
		// This ensures customers only see files they uploaded to the specific shop
		var err2 error
		printJobs, err2 = h.printJobRepo.GetByAccountEmailAndPartnerEmail(ctx, accountEmail, partnerEmail)
		if err2 != nil {
			log.Printf("ERROR: Failed to get print jobs - %v", err2)
			h.sendErrorResponse(w, http.StatusInternalServerError, "Internal error", "Failed to retrieve files")
			return
		}
		log.Printf("INFO: Found %d files for customer in shop '%s'", len(printJobs), shopName)
	} else {
		h.sendErrorResponse(w, http.StatusForbidden, "Forbidden", "Only customers and partners can view files")
		return
	}

	readyDir := h.agentHandler.GetReadyDir()
	fileList := []map[string]interface{}{}

	// Get Redis queue status for files
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

	// Process each print job
	for _, job := range printJobs {
		filePath := filepath.Join(readyDir, job.Filename)

		// Check if file exists in ready folder
		fileInfo, err := os.Stat(filePath)
		if err != nil {
			// File doesn't exist in ready folder (might be archived or deleted)
			log.Printf("DEBUG: File '%s' not found in ready folder (may be archived)", job.Filename)
			// Still include it in the list but mark as not available
			fileList = append(fileList, map[string]interface{}{
				"id":                        job.ID,
				"filename":                  job.Filename,
				"status":                    getStatus(job.Status, processingQueueMap[job.Filename], readyQueueMap[job.Filename]),
				"available":                 false,
				"preview_url":               fmt.Sprintf("/api/test-print/preview?filename=%s", job.Filename),
				"color":                     job.Color,
				"num_copies":                job.NumCopies,
				"p_type":                    job.PType,
				"page_options":              job.PageOptions,
				"back_to_back":              job.BackToBack,
				"created_at":                job.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			})
			continue
		}

		// Determine status based on Redis queues and database status
		status := getStatus(job.Status, processingQueueMap[job.Filename], readyQueueMap[job.Filename])

		// Get print_type (prefer PrintType over PType)
		var printType *string
		if job.PrintType != nil {
			printType = job.PrintType
		} else if job.PType != nil {
			printType = job.PType
		}
		
		fileList = append(fileList, map[string]interface{}{
			"id":                          job.ID,
			"filename":                    job.Filename,
			"size":                        fileInfo.Size(),
			"modified_at":                 fileInfo.ModTime().Format("2006-01-02T15:04:05Z07:00"),
			"status":                      status,
			"available":                   true,
			"preview_url":                 fmt.Sprintf("/api/test-print/preview?filename=%s", job.Filename),
			"color":                       job.Color,
			"num_copies":                  job.NumCopies,
			"p_type":                      job.PType,
			"print_type":                  printType,
			"crop_options":                job.PageOptions.CropOptions,
			"page_options":                job.PageOptions,
			"back_to_back":                job.BackToBack,
			"delete_after_print":         job.DeleteAfterPrint,
			"created_at":                  job.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		})
	}

	log.Printf("SUCCESS: Files listed for customer - Count: %d", len(fileList))

	h.sendJSONResponse(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"files":   fileList,
		"count":   len(fileList),
		"message": "Customer files retrieved successfully",
	})
}

// getStatus determines the file status based on database status and Redis queues
func getStatus(dbStatus *string, inProcessingQueue, inReadyQueue bool) string {
	// If in processing queue, it's being printed
	if inProcessingQueue {
		return "processing"
	}

	// If in ready queue, it's queued for printing
	if inReadyQueue {
		return "queued"
	}

	// Use database status if available
	if dbStatus != nil {
		return *dbStatus
	}

	// Default to pending
	return "pending"
}

