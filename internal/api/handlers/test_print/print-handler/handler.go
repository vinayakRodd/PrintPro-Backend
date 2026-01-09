package print_handler

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"print-pro-backend/internal/middleware/auth_middleware"
	"strconv"
	"strings"
)

// PrintFile handles print requests from partners - sends file directly to printer
func (h *PrintHandler) PrintFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.sendErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed", "Only POST method is allowed")
		return
	}

	// Get user from context (set by auth middleware)
	user, ok := auth_middleware.GetUserFromContext(r)
	if !ok {
		h.sendErrorResponse(w, http.StatusUnauthorized, "Unauthorized", "User not found in session")
		return
	}

	// Verify user is a partner
	if user.UserType != "partner" {
		h.sendErrorResponse(w, http.StatusForbidden, "Forbidden", "Only partners can print files")
		return
	}

	// Parse request body
	var req PrintRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("ERROR: Failed to decode request body - %v", err)
		h.sendErrorResponse(w, http.StatusBadRequest, "Invalid request body", "Failed to parse JSON: "+err.Error())
		return
	}

	// Log the raw request for debugging
	log.Printf("DEBUG: Print request received - Filename: '%s', Printer: '%s'", req.Filename, req.Printer)

	// Validate filename
	if req.Filename == "" {
		log.Printf("ERROR: Empty filename in print request")
		h.sendErrorResponse(w, http.StatusBadRequest, "Filename required", "Please provide a filename")
		return
	}

	// Sanitize filename (prevent path traversal)
	filename := filepath.Base(req.Filename) // Get only the base name
	filename = strings.ReplaceAll(filename, "..", "_")
	filename = strings.ReplaceAll(filename, "/", "_")
	filename = strings.ReplaceAll(filename, "\\", "_")

	log.Printf("DEBUG: Sanitized filename: '%s'", filename)

	// Look for file directly in ready folder (when partner clicks print)
	readyDir := h.agentHandler.GetReadyDir()
	readyFilePath := filepath.Join(readyDir, filename)
	log.Printf("DEBUG: Looking for file in ready folder: %s", readyFilePath)

	// Check if file exists in ready folder
	if _, err := os.Stat(readyFilePath); os.IsNotExist(err) {
		log.Printf("ERROR: File not found in ready folder: %s", filename)
		h.sendErrorResponse(w, http.StatusNotFound, "File not found", fmt.Sprintf("File '%s' not found in ready folder. Please ensure the file is uploaded first.", filename))
		return
	}

	// Get absolute file path
	absFilePath, err := filepath.Abs(readyFilePath)
	if err != nil {
		log.Printf("ERROR: Failed to get absolute path - %v", err)
		h.sendErrorResponse(w, http.StatusInternalServerError, "Internal error", "Failed to resolve file path")
		return
	}

	log.Printf("DEBUG: Absolute file path: %s", absFilePath)

	// Detect file type
	fileExt := strings.ToLower(filepath.Ext(absFilePath))
	isPDF := fileExt == ".pdf"
	isPNG := fileExt == ".png"
	isImage := isPNG || fileExt == ".jpg" || fileExt == ".jpeg"
	
	// Accept PDF and image files (PNG, JPG, JPEG)
	if !isPDF && !isImage {
		log.Printf("ERROR: Unsupported file type requested for printing: %s", fileExt)
		h.sendErrorResponse(w, http.StatusBadRequest, "Invalid file type", 
			fmt.Sprintf("Only PDF and image files (PNG, JPG, JPEG) can be printed. File type '%s' is not supported.", fileExt))
		return
	}

	pdfFileName := filepath.Base(absFilePath)
	ctx := r.Context()
	
	// OPTIMIZATION: Get printer_id directly from authenticated user (fast, single query)
	// No need to query print_job first - we already know the partner from auth
	var targetPrinterID string
	accountID, err := strconv.ParseInt(user.ID, 10, 64)
	if err == nil {
		// Get partner profile directly from account_id (we already have user.ID)
		partnerProfile, err := h.partnerProfileRepo.GetByAccountID(ctx, accountID)
		if err == nil && partnerProfile != nil {
			targetPrinterID = partnerProfile.PrinterID
			log.Printf("DEBUG: Found printer_id '%s' for partner account_id=%d", targetPrinterID, accountID)
		}
	}
	
	// OPTIMIZATION: Queue file in Redis FIRST (fast operation)
	log.Printf("INFO: Partner requested print - Queueing file in Redis - File: %s, Printer: %s, Partner: %s", pdfFileName, req.Printer, user.ID)
	
	if err := h.agentHandler.MoveToProcessing(pdfFileName); err != nil {
		log.Printf("ERROR: Failed to queue file in Redis: %v - File: %s, Partner: %s", err, pdfFileName, user.ID)
		h.sendErrorResponse(w, http.StatusInternalServerError, "File error", "Failed to queue file: "+err.Error())
		return
	}
	
	log.Printf("SUCCESS: File queued in Redis ready queue - Partner: %s, File: %s", user.ID, pdfFileName)
	
	// OPTIMIZATION: Send WebSocket notification IMMEDIATELY (don't wait for anything)
	// Use whatever printer_id the agent is actually connected with, not the database value
	if h.wsHub != nil {
		connectedPrinters := h.wsHub.ListConnectedPrinters()
		if len(connectedPrinters) > 0 {
			// Send to the first connected printer (or all if multiple)
			// The agent connects with its own printer_id, so use that
			actualPrinterID := connectedPrinters[0]
			log.Printf("DEBUG: Using connected printer_id: '%s' (database had: '%s')", actualPrinterID, targetPrinterID)
			
			// Send notification message with filename
			notification := map[string]interface{}{
				"action": "print_job_available",
				"payload": map[string]interface{}{
					"filename": pdfFileName,
				},
			}
			
			notificationJSON, err := json.Marshal(notification)
			if err != nil {
				log.Printf("ERROR: Failed to marshal WebSocket notification: %v", err)
			} else {
				// Send to all connected printers (in case there are multiple)
				successCount := 0
				for _, printerID := range connectedPrinters {
					if err := h.wsHub.SendToPrinter(printerID, notificationJSON); err != nil {
						log.Printf("WARNING: Failed to send WebSocket notification to '%s': %v", printerID, err)
					} else {
						log.Printf("SUCCESS: WebSocket notification sent to printer_id: '%s' for file: %s", printerID, pdfFileName)
						successCount++
					}
				}
				if successCount == 0 {
					log.Printf("WARNING: Failed to send WebSocket notification to any connected printer (agent will poll instead)")
				}
			}
		} else {
			log.Printf("DEBUG: No WebSocket connections currently active (agent will poll Redis queue)")
		}
	} else {
		log.Printf("DEBUG: WebSocket hub is nil - cannot send notification")
	}

	// Return success response with processing status
	h.sendJSONResponse(w, http.StatusOK, map[string]interface{}{
		"success":  true,
		"message":  "Print queued",
		"filename": pdfFileName,
		"printer":  req.Printer,
		"status":   "processing",
	})
}

// QueueFile moves a file from ready folder to processing folder (queues it for agent)
func (h *PrintHandler) QueueFile(w http.ResponseWriter, r *http.Request) {
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
		h.sendErrorResponse(w, http.StatusForbidden, "Forbidden", "Only partners can queue files")
		return
	}

	// Parse request body
	var req PrintRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("ERROR: Failed to decode request body - %v", err)
		h.sendErrorResponse(w, http.StatusBadRequest, "Invalid request body", "Failed to parse JSON: "+err.Error())
		return
	}

	// Validate filename
	if req.Filename == "" {
		h.sendErrorResponse(w, http.StatusBadRequest, "Filename required", "Please provide a filename")
		return
	}

	// Sanitize filename
	filename := filepath.Base(req.Filename)
	filename = strings.ReplaceAll(filename, "..", "_")
	filename = strings.ReplaceAll(filename, "/", "_")
	filename = strings.ReplaceAll(filename, "\\", "_")

	log.Printf("INFO: Partner accepted/queued file for agent - Partner: %s, File: %s", user.ID, filename)

	// Move file from ready to processing folder (now agent can fetch it)
	if err := h.agentHandler.MoveToProcessing(filename); err != nil {
		log.Printf("ERROR: Failed to queue file: %v", err)
		h.sendErrorResponse(w, http.StatusBadRequest, "Failed to queue file", err.Error())
		return
	}

	log.Printf("SUCCESS: File queued for agent - Partner: %s, File: %s (agent can now fetch)", user.ID, filename)

	h.sendJSONResponse(w, http.StatusOK, map[string]interface{}{
		"success":  true,
		"message":  "File queued for printing",
		"filename": filename,
		"status":   "queued",
	})
}


