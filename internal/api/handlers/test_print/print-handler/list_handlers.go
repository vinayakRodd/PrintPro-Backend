package print_handler

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"print-pro-backend/internal/middleware/auth_middleware"
	"strings"
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

	readyDir := h.agentHandler.GetReadyDir()

	// Ensure directory exists
	os.MkdirAll(readyDir, 0755)

	fileList := []map[string]interface{}{}
	readyCount := 0
	processingCount := 0

	// Get filenames from Redis queues
	ctx := r.Context()
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

	// Read all files from ready folder
	readyFiles, err := os.ReadDir(readyDir)
	if err == nil {
		for _, file := range readyFiles {
			if file.IsDir() || strings.ToLower(filepath.Ext(file.Name())) != ".pdf" {
				continue
			}

			filename := file.Name()
			info, err := file.Info()
			if err != nil {
				continue
			}

			// Determine status based on Redis queues
			var status string
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

			fileList = append(fileList, map[string]interface{}{
				"filename":    filename,
				"size":        info.Size(),
				"modified_at": info.ModTime().Format("2006-01-02T15:04:05Z07:00"),
				"status":      status,
				"preview_url": fmt.Sprintf("/api/test-print/preview?filename=%s", filename),
			})
		}
	}

	log.Printf("SUCCESS: Files listed from Redis queues - Partner: %s, Ready: %d, Processing: %d, Total: %d",
		user.ID, readyCount, processingCount, len(fileList))

	h.sendJSONResponse(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"files":   fileList,
		"count":   len(fileList),
		"message": "PDFs from Redis queues and ready folder",
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

	log.Printf("INFO: ListPrinters called - Partner: %s, Printer count from partner agent: %d", user.ID, len(printers))

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

	log.Printf("SUCCESS: Sending printer list to frontend - Partner: %s, Count: %d", user.ID, len(printers))

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

	log.Printf("SUCCESS: Printer list sent to frontend - Partner: %s, Count: %d (EXACTLY as received from partner agent)", user.ID, len(printers))
}

