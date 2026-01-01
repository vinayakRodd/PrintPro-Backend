package test_print

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"print-pro-backend/internal/middleware/auth_middleware"
	"strings"
	"time"
)

// UploadHandler handles file uploads from customers
type UploadHandler struct {
	uploadDir string
}

// NewUploadHandler creates a new upload handler
func NewUploadHandler(uploadDir string) *UploadHandler {
	return &UploadHandler{
		uploadDir: uploadDir,
	}
}

// UploadFile handles file upload requests from customers
func (h *UploadHandler) UploadFile(w http.ResponseWriter, r *http.Request) {
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

	// Verify user is a customer
	if user.UserType != "customer" {
		h.sendErrorResponse(w, http.StatusForbidden, "Forbidden", "Only customers can upload files")
		return
	}

	// Check Content-Type header
	contentType := r.Header.Get("Content-Type")
	log.Printf("DEBUG: Upload request - Method: %s, Content-Type: %s, Content-Length: %s", 
		r.Method, contentType, r.Header.Get("Content-Length"))
	
	// Check if Content-Type is multipart/form-data (must start with "multipart/form-data")
	if contentType == "" {
		log.Printf("ERROR: Missing Content-Type header")
		h.sendErrorResponse(w, http.StatusBadRequest, "Invalid request", "Missing Content-Type header. Request must be multipart/form-data.")
		return
	}
	
	if !strings.HasPrefix(contentType, "multipart/form-data") {
		log.Printf("ERROR: Invalid Content-Type - Expected multipart/form-data, got: %s", contentType)
		h.sendErrorResponse(w, http.StatusBadRequest, "Invalid Content-Type", 
			fmt.Sprintf("Request must be multipart/form-data. Received: %s. Please use FormData in your frontend.", contentType))
		return
	}
	
	// Parse multipart form (max 10MB)
	err := r.ParseMultipartForm(10 << 20) // 10MB
	if err != nil {
		log.Printf("ERROR: Failed to parse multipart form - Content-Type: %s, Error: %v", contentType, err)
		h.sendErrorResponse(w, http.StatusBadRequest, "Invalid form data", 
			fmt.Sprintf("Failed to parse multipart form. Content-Type: %s. Error: %v", contentType, err))
		return
	}
	
	log.Printf("DEBUG: Multipart form parsed successfully")

	// Get file from form
	file, header, err := r.FormFile("file")
	if err != nil {
		log.Printf("ERROR: Failed to get file from form - %v", err)
		h.sendErrorResponse(w, http.StatusBadRequest, "File required", "Please provide a file in the 'file' field")
		return
	}
	defer file.Close()

	// Validate file size (max 10MB)
	if header.Size > 10<<20 {
		h.sendErrorResponse(w, http.StatusBadRequest, "File too large", "File size exceeds 10MB limit")
		return
	}

	// Sanitize filename
	originalFilename := header.Filename
	ext := filepath.Ext(originalFilename)
	nameWithoutExt := strings.TrimSuffix(originalFilename, ext)
	
	// Remove dangerous characters
	nameWithoutExt = strings.ReplaceAll(nameWithoutExt, "..", "_")
	nameWithoutExt = strings.ReplaceAll(nameWithoutExt, "/", "_")
	nameWithoutExt = strings.ReplaceAll(nameWithoutExt, "\\", "_")
	
	// Create unique filename: timestamp_userid_originalname
	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("%s_%s_%s%s", timestamp, user.ID, nameWithoutExt, ext)

	// Build file path
	filePath := filepath.Join(h.uploadDir, filename)

	// Create upload directory if it doesn't exist
	if err := os.MkdirAll(h.uploadDir, 0755); err != nil {
		log.Printf("ERROR: Failed to create upload directory - %v", err)
		h.sendErrorResponse(w, http.StatusInternalServerError, "Internal error", "Failed to create upload directory")
		return
	}

	// Create file on disk
	dst, err := os.Create(filePath)
	if err != nil {
		log.Printf("ERROR: Failed to create file - %v", err)
		h.sendErrorResponse(w, http.StatusInternalServerError, "Internal error", "Failed to save file")
		return
	}
	defer dst.Close()

	// Copy file content
	_, err = io.Copy(dst, file)
	if err != nil {
		log.Printf("ERROR: Failed to write file - %v", err)
		os.Remove(filePath) // Clean up on error
		h.sendErrorResponse(w, http.StatusInternalServerError, "Internal error", "Failed to save file")
		return
	}

	log.Printf("SUCCESS: File uploaded - Customer: %s, File: %s, Size: %d bytes", user.ID, filename, header.Size)

	// Return success response
	h.sendJSONResponse(w, http.StatusOK, map[string]interface{}{
		"success":  true,
		"message":  "File uploaded successfully",
		"filename": filename,
		"size":     header.Size,
	})
}

// sendJSONResponse sends a JSON response
func (h *UploadHandler) sendJSONResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("ERROR: Failed to encode JSON response - %v", err)
	}
}

// sendErrorResponse sends an error JSON response
func (h *UploadHandler) sendErrorResponse(w http.ResponseWriter, statusCode int, message, error string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": false,
		"message": message,
		"error":   error,
	})
}

