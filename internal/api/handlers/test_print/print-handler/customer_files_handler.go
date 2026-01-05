package print_handler

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"print-pro-backend/internal/middleware/auth_middleware"
	"strconv"
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

	// Verify user is a customer
	if user.UserType != "customer" {
		h.sendErrorResponse(w, http.StatusForbidden, "Forbidden", "Only customers can view their files")
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

	// Get shop_name query parameter (REQUIRED)
	// SECURITY: shop_name is mandatory - customers can only view files for a specific shop
	shopName := r.URL.Query().Get("shop_name")
	if shopName == "" {
		log.Printf("ERROR: shop_name parameter is required but not provided")
		h.sendErrorResponse(w, http.StatusBadRequest, "Shop name required", "Please provide shop_name query parameter to view files for a specific shop")
		return
	}

	shopName = strings.TrimSpace(shopName)
	log.Printf("INFO: Customer requested files for shop: %s (account_id: %d)", shopName, accountID)

	// Get partner_id from shop_name
	partnerProfile, err := h.partnerProfileRepo.GetByShopName(ctx, shopName)
	if err != nil {
		log.Printf("ERROR: Shop not found: %s - %v", shopName, err)
		h.sendErrorResponse(w, http.StatusBadRequest, "Invalid shop", fmt.Sprintf("Shop '%s' not found", shopName))
		return
	}

	partnerID := partnerProfile.ID
	log.Printf("INFO: Querying files for customer account_id: %d AND shop '%s' (partner_id: %d)", accountID, shopName, partnerID)

	// SECURITY: Query database with BOTH account_id AND partner_id - database-level filtering
	// This ensures customers only see files they uploaded to the specific shop
	printJobs, err := h.printJobRepo.GetByAccountIDAndPartnerID(ctx, accountID, partnerID)
	if err != nil {
		log.Printf("ERROR: Failed to get print jobs for account_id %d and partner_id %d: %v", accountID, partnerID, err)
		h.sendErrorResponse(w, http.StatusInternalServerError, "Internal error", "Failed to retrieve files")
		return
	}
	log.Printf("INFO: Found %d files for customer account_id: %d in shop '%s' (partner_id: %d)", len(printJobs), accountID, shopName, partnerID)

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
				"id":          job.ID,
				"filename":    job.Filename,
				"status":      getStatus(job.Status, processingQueueMap[job.Filename], readyQueueMap[job.Filename]),
				"available":   false,
				"preview_url": fmt.Sprintf("/api/test-print/preview?filename=%s", job.Filename),
				"color":       job.Color,
				"num_copies":  job.NumCopies,
				"p_type":      job.PType,
				"created_at":  job.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			})
			continue
		}

		// Determine status based on Redis queues and database status
		status := getStatus(job.Status, processingQueueMap[job.Filename], readyQueueMap[job.Filename])

		fileList = append(fileList, map[string]interface{}{
			"id":          job.ID,
			"filename":    job.Filename,
			"size":        fileInfo.Size(),
			"modified_at": fileInfo.ModTime().Format("2006-01-02T15:04:05Z07:00"),
			"status":      status,
			"available":   true,
			"preview_url": fmt.Sprintf("/api/test-print/preview?filename=%s", job.Filename),
			"color":       job.Color,
			"num_copies":  job.NumCopies,
			"p_type":      job.PType,
			"created_at":  job.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		})
	}

	log.Printf("SUCCESS: Files listed for customer - Account: %s, Count: %d", user.ID, len(fileList))

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

