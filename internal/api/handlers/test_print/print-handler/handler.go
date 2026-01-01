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

	// Build file path
	filePath := filepath.Join(h.uploadDir, filename)
	log.Printf("DEBUG: Looking for file at: %s", filePath)

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		log.Printf("ERROR: File not found - %s", filename)
		h.sendErrorResponse(w, http.StatusNotFound, "File not found", fmt.Sprintf("File '%s' does not exist", filename))
		return
	}

	// Get absolute file path
	absFilePath, err := filepath.Abs(filePath)
	if err != nil {
		log.Printf("ERROR: Failed to get absolute path - %v", err)
		h.sendErrorResponse(w, http.StatusInternalServerError, "Internal error", "Failed to resolve file path")
		return
	}

	log.Printf("DEBUG: Absolute file path: %s", absFilePath)

	// Detect file type
	fileExt := strings.ToLower(filepath.Ext(absFilePath))
	isPDF := fileExt == ".pdf"
	
	// Check if it's an image file (screenshot) - reject these
	imageExts := []string{".png", ".jpg", ".jpeg", ".gif", ".bmp", ".webp", ".svg"}
	isImage := false
	for _, imgExt := range imageExts {
		if fileExt == imgExt {
			isImage = true
			break
		}
	}
	
	log.Printf("DEBUG: File extension: '%s', Is PDF: %v, Is Image: %v", fileExt, isPDF, isImage)
	
	// If an image file is sent (screenshot), find and print the most recent PDF instead
	if isImage {
		log.Printf("WARNING: Image file detected (screenshot). Finding most recent PDF file instead...")
		pdfPath, err := h.findMostRecentPDF()
		if err != nil {
			log.Printf("ERROR: Failed to find PDF file - %v", err)
			h.sendErrorResponse(w, http.StatusBadRequest, "Invalid file type", 
				"Screenshots/images cannot be printed. Please select a PDF file to print. Error: "+err.Error())
			return
		}
		log.Printf("INFO: Found PDF file to print instead: %s", filepath.Base(pdfPath))
		absFilePath = pdfPath
		isPDF = true
	}
	
	// Ensure we only print PDFs (reject other non-PDF files if not already handled)
	if !isPDF {
		log.Printf("ERROR: Non-PDF file type requested for printing: %s", fileExt)
		h.sendErrorResponse(w, http.StatusBadRequest, "Invalid file type", 
			fmt.Sprintf("Only PDF files can be printed. File type '%s' is not supported.", fileExt))
		return
	}

	// Print to printer using Windows native method
	if isWindows() {
		// Always use PDF printing method for PDFs
		err = h.printWindowsPDF(absFilePath, req.Printer)
	} else {
		err = h.printUnixFile(absFilePath, req.Printer)
	}

	if err != nil {
		log.Printf("ERROR: Failed to print file - %v", err)
		h.sendErrorResponse(w, http.StatusInternalServerError, "Print failed", err.Error())
		return
	}

	log.Printf("SUCCESS: File printed - Partner: %s, File: %s, Printer: %s", user.ID, filename, req.Printer)

	// Return success response
	h.sendJSONResponse(w, http.StatusOK, map[string]interface{}{
		"success":  true,
		"message":  "File sent to printer successfully",
		"filename": filename,
		"printer":  req.Printer,
	})
}

// ListFiles lists all available files for printing (for partners)
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

	// Read directory
	files, err := os.ReadDir(h.uploadDir)
	if err != nil {
		log.Printf("ERROR: Failed to read upload directory - %v", err)
		h.sendErrorResponse(w, http.StatusInternalServerError, "Failed to list files", err.Error())
		return
	}

	// Get file info
	fileList := []map[string]interface{}{}
	for _, file := range files {
		if file.IsDir() {
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
		})
	}

	log.Printf("SUCCESS: Files listed - Partner: %s, Count: %d", user.ID, len(fileList))

	h.sendJSONResponse(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"files":   fileList,
		"count":   len(fileList),
	})
}

// ListPrinters lists all available printers on the system
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

	// Get list of printers based on OS
	var printers []map[string]interface{}
	var err error

	if isWindows() {
		printers, err = h.getWindowsPrinters()
	} else {
		printers, err = h.getUnixPrinters()
	}

	if err != nil {
		log.Printf("ERROR: Failed to get printers - %v", err)
		h.sendErrorResponse(w, http.StatusInternalServerError, "Failed to list printers", err.Error())
		return
	}

	log.Printf("SUCCESS: Printers listed - Partner: %s, Count: %d", user.ID, len(printers))

		h.sendJSONResponse(w, http.StatusOK, map[string]interface{}{
			"success":  true,
			"printers": printers,
			"count":    len(printers),
		})
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

