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
	
	// Ensure we only print PDFs
	if !isPDF {
		log.Printf("ERROR: Non-PDF file type requested for printing: %s", fileExt)
		h.sendErrorResponse(w, http.StatusBadRequest, "Invalid file type", 
			fmt.Sprintf("Only PDF files can be printed. File type '%s' is not supported.", fileExt))
		return
	}

	// Queue PDF file for partner agent when frontend explicitly requests print
	// Logic: Pick PDF from ready folder (by filename) → Move to processing folder (queued)
	// The partner agent will fetch this file via /api/partner-agent/fetch-job endpoint
	pdfFileName := filepath.Base(absFilePath)
	
	log.Printf("INFO: Partner requested print - Moving file from ready to processing folder - File: %s, Printer: %s", pdfFileName, req.Printer)
	
	// Move file from ready folder to processing folder (queued)
	if err := h.agentHandler.MoveToProcessing(pdfFileName); err != nil {
		log.Printf("ERROR: Failed to move file from ready to processing folder: %v", err)
		h.sendErrorResponse(w, http.StatusInternalServerError, "File error", "Failed to queue file: "+err.Error())
		return
	}
	
	log.Printf("SUCCESS: File moved from ready to processing folder - Partner: %s, File: %s (processing, agent can now fetch)", user.ID, pdfFileName)

	// Return success response with processing status
	h.sendJSONResponse(w, http.StatusOK, map[string]interface{}{
		"success":  true,
		"message":  "Print queued",
		"filename": pdfFileName,
		"printer":  req.Printer,
		"status":   "processing",
	})
}

// ListFiles lists all PDFs from ready and processing folders (NOT archived)
// - Ready folder: status "ready" (show "Print" button)
// - Processing folder: status "queued" (show "Queued" status)
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
	processingDir := h.agentHandler.GetProcessingDir()
	
	// Ensure directories exist
	os.MkdirAll(readyDir, 0755)
	os.MkdirAll(processingDir, 0755)
	
	fileList := []map[string]interface{}{}
	readyCount := 0
	queuedCount := 0

	// Read from ready folder (status: "ready" - show Print button)
	readyFiles, err := os.ReadDir(readyDir)
	if err == nil {
		for _, file := range readyFiles {
			if file.IsDir() || strings.ToLower(filepath.Ext(file.Name())) != ".pdf" {
				continue
			}

			info, err := file.Info()
			if err != nil {
				continue
			}

			fileList = append(fileList, map[string]interface{}{
				"filename":    file.Name(),
				"size":        info.Size(),
				"modified_at": info.ModTime().Format("2006-01-02T15:04:05Z07:00"),
				"status":      "ready", // Status: ready to print (show Print button)
			})
			readyCount++
		}
	}

	// Read from processing folder (status: "processing" - show Processing status)
	processingFiles, err := os.ReadDir(processingDir)
	if err == nil {
		for _, file := range processingFiles {
			if file.IsDir() || strings.ToLower(filepath.Ext(file.Name())) != ".pdf" {
				continue
			}

			info, err := file.Info()
			if err != nil {
				continue
			}

			fileList = append(fileList, map[string]interface{}{
				"filename":    file.Name(),
				"size":        info.Size(),
				"modified_at": info.ModTime().Format("2006-01-02T15:04:05Z07:00"),
				"status":      "processing", // Status: processing (show Processing button/status)
			})
			queuedCount++
		}
	}

	log.Printf("SUCCESS: Files listed - Partner: %s, Ready: %d, Processing: %d, Total: %d", 
		user.ID, readyCount, queuedCount, len(fileList))

	h.sendJSONResponse(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"files":   fileList,
		"count":   len(fileList),
		"message": "PDFs from ready and processing folders",
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

// ListPrinters lists all available printers from the synced partner agent list
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

	// Get synced printer list from partner agent (instead of detecting locally)
	printers := h.agentHandler.GetSyncedPrinters()

	log.Printf("DEBUG: ListPrinters called - Partner: %s, Printer count: %d", user.ID, len(printers))
	
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

	// Prepare response
	response := map[string]interface{}{
		"success":  true,
		"printers": printers,
		"count":    len(printers),
		"message":  "Printers from synced partner agent list",
	}
	
	// Log the response being sent
	responseJSON, _ := json.MarshalIndent(response, "", "  ")
	log.Printf("DEBUG: Response being sent to frontend:\n%s", string(responseJSON))
	
	h.sendJSONResponse(w, http.StatusOK, response)
	
	log.Printf("SUCCESS: Printers list sent to frontend - Partner: %s, Count: %d", user.ID, len(printers))
}

// findMostRecentPDF finds the most recently modified PDF file in the upload directory
func (h *PrintHandler) findMostRecentPDF() (string, error) {
	// Read directory
	files, err := os.ReadDir(h.uploadDir)
	if err != nil {
		return "", fmt.Errorf("failed to read upload directory: %v", err)
	}

	var mostRecentPDF string
	var mostRecentTime int64 = 0

	// Find the most recent PDF file
	for _, file := range files {
		if file.IsDir() {
			continue
		}

		// Check if it's a PDF
		if strings.ToLower(filepath.Ext(file.Name())) != ".pdf" {
			continue
		}

		// Get file info
		info, err := file.Info()
		if err != nil {
			continue
		}

		// Check if this is the most recent
		modTime := info.ModTime().Unix()
		if modTime > mostRecentTime {
			mostRecentTime = modTime
			mostRecentPDF = file.Name()
		}
	}

	if mostRecentPDF == "" {
		return "", fmt.Errorf("no PDF files found in upload directory")
	}

	// Return absolute path
	pdfPath := filepath.Join(h.uploadDir, mostRecentPDF)
	absPath, err := filepath.Abs(pdfPath)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path: %v", err)
	}

	return absPath, nil
}

