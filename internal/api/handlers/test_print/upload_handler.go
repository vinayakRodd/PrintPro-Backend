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
	uploadDir    string
	agentHandler interface {
		QueueJobForAgent(sourceFilePath string) error
	}
}

// NewUploadHandler creates a new upload handler
func NewUploadHandler(uploadDir string, agentHandler interface {
	QueueJobForAgent(sourceFilePath string) error
}) *UploadHandler {
	return &UploadHandler{
		uploadDir:    uploadDir,
		agentHandler: agentHandler,
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

	// Save file directly to ready folder (as per requirements)
	// QueueJobForAgent will handle copying to ready folder and pushing to Redis
	// Build temporary file path first (in uploadDir)
	tempFilePath := filepath.Join(h.uploadDir, filename)

	// Create upload directory if it doesn't exist (for temp file)
	if err := os.MkdirAll(h.uploadDir, 0755); err != nil {
		log.Printf("ERROR: Failed to create upload directory - %v", err)
		h.sendErrorResponse(w, http.StatusInternalServerError, "Internal error", "Failed to create upload directory")
		return
	}

	// Create temporary file on disk
	dst, err := os.Create(tempFilePath)
	if err != nil {
		log.Printf("ERROR: Failed to create file - %v", err)
		h.sendErrorResponse(w, http.StatusInternalServerError, "Internal error", "Failed to save file")
		return
	}
	defer dst.Close()

	// Copy file content to temporary location
	_, err = io.Copy(dst, file)
	if err != nil {
		log.Printf("ERROR: Failed to write file - %v", err)
		os.Remove(tempFilePath) // Clean up on error
		h.sendErrorResponse(w, http.StatusInternalServerError, "Internal error", "Failed to save file")
		return
	}

	// Queue job to ready folder and push to Redis
	// This will copy file to ready folder and push filename to Redis list
	if h.agentHandler != nil {
		if err := h.agentHandler.QueueJobForAgent(tempFilePath); err != nil {
			log.Printf("ERROR: Failed to queue job to ready folder: %v", err)
			os.Remove(tempFilePath) // Clean up temp file on error
			h.sendErrorResponse(w, http.StatusInternalServerError, "Internal error", "Failed to queue file: "+err.Error())
			return
		}
		// Remove temp file after successful copy to ready folder
		os.Remove(tempFilePath)
	} else {
		// If no agent handler, keep file in uploadDir (fallback)
		log.Printf("WARNING: No agent handler available, file saved to upload directory only")
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

