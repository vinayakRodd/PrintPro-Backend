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
	// Logic: Ensure file is in Redis ready queue → Agent will fetch via RPOPLPUSH
	// The partner agent will fetch this file via /api/partner-agent/fetch-job endpoint
	pdfFileName := filepath.Base(absFilePath)
	
	log.Printf("INFO: Partner requested print - Ensuring file is in Redis ready queue - File: %s, Printer: %s, Partner: %s", pdfFileName, req.Printer, user.ID)
	
	// Ensure file is in Redis ready queue (will check for duplicates)
	if err := h.agentHandler.MoveToProcessing(pdfFileName); err != nil {
		log.Printf("ERROR: Failed to queue file in Redis: %v - File: %s, Partner: %s", err, pdfFileName, user.ID)
		h.sendErrorResponse(w, http.StatusInternalServerError, "File error", "Failed to queue file: "+err.Error())
		return
	}
	
	log.Printf("SUCCESS: File queued in Redis ready queue - Partner: %s, File: %s (agent can now fetch via RPOPLPUSH)", user.ID, pdfFileName)

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


