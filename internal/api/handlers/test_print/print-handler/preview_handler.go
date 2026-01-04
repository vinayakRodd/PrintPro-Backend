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

// PreviewPDF serves PDF and image files for viewing in browser/iframe
// GET /api/test-print/preview?filename=<filename>
func (h *PrintHandler) PreviewPDF(w http.ResponseWriter, r *http.Request) {
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

	// Get filename from query parameter
	filename := r.URL.Query().Get("filename")
	if filename == "" {
		h.sendErrorResponse(w, http.StatusBadRequest, "Filename required", "Please provide a filename in query parameter")
		return
	}

	log.Printf("DEBUG: PreviewPDF request - User: %s (Type: %s), Filename: %s", user.ID, user.UserType, filename)

	// Sanitize filename (prevent path traversal)
	sanitizedFilename := filepath.Base(filename) // Get only the base name
	sanitizedFilename = strings.ReplaceAll(sanitizedFilename, "..", "_")
	sanitizedFilename = strings.ReplaceAll(sanitizedFilename, "/", "_")
	sanitizedFilename = strings.ReplaceAll(sanitizedFilename, "\\", "_")

	// If user is a customer, verify they own the file
	if user.UserType == "customer" {
		ctx := r.Context()
		accountID, err := strconv.ParseInt(user.ID, 10, 64)
		if err != nil {
			log.Printf("ERROR: Failed to parse user ID '%s' - %v", user.ID, err)
			h.sendErrorResponse(w, http.StatusBadRequest, "Invalid user ID", "Invalid account ID format")
			return
		}

		// Check if the file belongs to this customer
		job, err := h.printJobRepo.GetByFilename(ctx, sanitizedFilename)
		if err != nil {
			log.Printf("ERROR: File not found in database or access denied - User: %s, File: %s, Error: %v", user.ID, sanitizedFilename, err)
			h.sendErrorResponse(w, http.StatusForbidden, "Access denied", "You do not have permission to view this file")
			return
		}

		// Verify the file belongs to this customer
		if job.AccountID == nil || *job.AccountID != accountID {
			log.Printf("ERROR: Customer trying to access file that doesn't belong to them - User: %s, File: %s, File Owner: %v", user.ID, sanitizedFilename, job.AccountID)
			h.sendErrorResponse(w, http.StatusForbidden, "Access denied", "You do not have permission to view this file")
			return
		}

		log.Printf("DEBUG: Customer access verified - User: %s owns file: %s", user.ID, sanitizedFilename)
	}
	// Partners can view any file (no ownership check needed)

	// Get file extension for content type detection
	ext := strings.ToLower(filepath.Ext(sanitizedFilename))
	isPDF := ext == ".pdf"
	isImage := ext == ".jpg" || ext == ".jpeg" || ext == ".png" || ext == ".gif" || ext == ".bmp" || ext == ".webp"

	// Check file in ready folder first, then processing folder
	readyDir := h.agentHandler.GetReadyDir()
	processingDir := h.agentHandler.GetProcessingDir()

	var filePath string
	var found bool

	// Check ready folder
	readyPath := filepath.Join(readyDir, sanitizedFilename)
	if _, err := os.Stat(readyPath); err == nil {
		filePath = readyPath
		found = true
		log.Printf("DEBUG: File found in ready folder: %s", readyPath)
	}

	// Check processing folder if not found in ready
	if !found {
		processingPath := filepath.Join(processingDir, sanitizedFilename)
		if _, err := os.Stat(processingPath); err == nil {
			filePath = processingPath
			found = true
			log.Printf("DEBUG: File found in processing folder: %s", processingPath)
		}
	}

	if !found {
		log.Printf("ERROR: File not found: %s", sanitizedFilename)
		h.sendErrorResponse(w, http.StatusNotFound, "File not found", fmt.Sprintf("File '%s' not found in ready or processing folders", sanitizedFilename))
		return
	}

	// Get absolute path for security
	absFilePath, err := filepath.Abs(filePath)
	if err != nil {
		log.Printf("ERROR: Failed to get absolute path - %v", err)
		h.sendErrorResponse(w, http.StatusInternalServerError, "Internal error", "Failed to resolve file path")
		return
	}

	// Verify the file is within the allowed directories (additional security check)
	readyAbs, _ := filepath.Abs(readyDir)
	processingAbs, _ := filepath.Abs(processingDir)

	if !strings.HasPrefix(absFilePath, readyAbs) && !strings.HasPrefix(absFilePath, processingAbs) {
		log.Printf("ERROR: Path traversal attempt detected - File: %s", absFilePath)
		h.sendErrorResponse(w, http.StatusForbidden, "Access denied", "Invalid file path")
		return
	}

	// Open file
	file, err := os.Open(absFilePath)
	if err != nil {
		log.Printf("ERROR: Failed to open file - %v", err)
		h.sendErrorResponse(w, http.StatusInternalServerError, "Internal error", "Failed to open file")
		return
	}
	defer file.Close()

	// Get file info for Content-Length header
	fileInfo, err := file.Stat()
	if err != nil {
		log.Printf("ERROR: Failed to get file info - %v", err)
		h.sendErrorResponse(w, http.StatusInternalServerError, "Internal error", "Failed to get file information")
		return
	}

	// Set headers based on file type
	var contentType string
	if isPDF {
		contentType = "application/pdf"
	} else if isImage {
		// Set appropriate content type for images
		switch ext {
		case ".jpg", ".jpeg":
			contentType = "image/jpeg"
		case ".png":
			contentType = "image/png"
		case ".gif":
			contentType = "image/gif"
		case ".bmp":
			contentType = "image/bmp"
		case ".webp":
			contentType = "image/webp"
		default:
			contentType = "image/jpeg"
		}
	} else {
		// Set content type for other document types
		switch ext {
		case ".doc":
			contentType = "application/msword"
		case ".docx":
			contentType = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
		case ".xls":
			contentType = "application/vnd.ms-excel"
		case ".xlsx":
			contentType = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
		case ".txt":
			contentType = "text/plain"
		case ".rtf":
			contentType = "application/rtf"
		case ".odt":
			contentType = "application/vnd.oasis.opendocument.text"
		default:
			// For unknown types, use octet-stream (download instead of preview)
			contentType = "application/octet-stream"
		}
	}
	
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`inline; filename="%s"`, sanitizedFilename))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", fileInfo.Size()))
	w.Header().Set("Cache-Control", "private, max-age=3600")

	log.Printf("SUCCESS: Serving file for preview - User: %s, File: %s, Type: %s, Size: %d bytes", user.ID, sanitizedFilename, contentType, fileInfo.Size())

	// Stream the PDF file
	http.ServeContent(w, r, sanitizedFilename, fileInfo.ModTime(), file)
}

