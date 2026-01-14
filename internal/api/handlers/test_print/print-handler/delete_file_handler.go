package print_handler

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"print-pro-backend/internal/middleware/auth_middleware"
	"strings"
)

// DeleteFileRequest represents the request to delete a file
type DeleteFileRequest struct {
	Filename string `json:"filename"`
}

// DeleteFile handles requests to delete a file uploaded by the customer
// DELETE /api/test-print/delete
// Deletes file from database, Redis queues, and filesystem
func (h *PrintHandler) DeleteFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete && r.Method != http.MethodPost {
		h.sendErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed", "Only DELETE or POST method is allowed")
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
		h.sendErrorResponse(w, http.StatusForbidden, "Forbidden", "Only customers can delete their files")
		return
	}

	// Parse request body
	var req DeleteFileRequest
	if r.Method == http.MethodPost {
		// Support POST for easier frontend usage - try JSON body first
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			// If JSON fails, try form data
			if parseErr := r.ParseForm(); parseErr == nil {
				req.Filename = r.FormValue("filename")
			} else {
				log.Printf("ERROR: Failed to decode request body - %v", err)
				h.sendErrorResponse(w, http.StatusBadRequest, "Invalid request body", "Failed to parse request")
				return
			}
		}
	} else {
		// DELETE method - get filename from query parameter
		req.Filename = r.URL.Query().Get("filename")
	}

	// Validate filename
	if req.Filename == "" {
		log.Printf("ERROR: Empty filename in delete request")
		h.sendErrorResponse(w, http.StatusBadRequest, "Filename required", "Please provide a filename")
		return
	}

	// Sanitize filename (prevent path traversal)
	filename := filepath.Base(req.Filename)
	filename = strings.ReplaceAll(filename, "..", "_")
	filename = strings.ReplaceAll(filename, "/", "_")
	filename = strings.ReplaceAll(filename, "\\", "_")

	log.Printf("INFO: Delete request - Customer authorized, File: %s", filename)

	ctx := r.Context()

	// Get account_email from user.ID (user.ID is the email)
	accountEmail := user.ID

	// Step 1: Delete from database (only if file belongs to this customer)
	if err := h.printJobRepo.DeleteByFilenameAndAccountEmail(ctx, filename, accountEmail); err != nil {
		log.Printf("ERROR: Failed to delete print job from database: %v", err)
		h.sendErrorResponse(w, http.StatusNotFound, "File not found", "File not found or you don't have permission to delete it")
		return
	}
	log.Printf("SUCCESS: Print job deleted from database - File: %s", filename)

	// Step 2: Remove from Redis queues (ready and processing)
	if h.agentHandler != nil {
		if err := h.agentHandler.RemoveFromQueues(ctx, filename); err != nil {
			log.Printf("WARNING: Failed to remove from Redis queues: %v (file still deleted from DB)", err)
			// Don't fail the request - file is already removed from database
		} else {
			log.Printf("SUCCESS: File removed from Redis queues - File: %s", filename)
		}
	}

	// Step 3: Delete physical file from ready folder
	readyDir := h.agentHandler.GetReadyDir()
	filePath := filepath.Join(readyDir, filename)
	
	if _, err := os.Stat(filePath); err == nil {
		// File exists, delete it
		if err := os.Remove(filePath); err != nil {
			log.Printf("WARNING: Failed to delete physical file: %v (file removed from DB and Redis)", err)
			// Don't fail the request - file is already removed from DB and Redis
		} else {
			log.Printf("SUCCESS: Physical file deleted - File: %s", filename)
		}
	} else {
		log.Printf("INFO: Physical file not found (may have been deleted already) - File: %s", filename)
	}

	log.Printf("SUCCESS: File deleted completely - File: %s", filename)

	h.sendJSONResponse(w, http.StatusOK, map[string]interface{}{
		"success":  true,
		"message":  "File deleted successfully",
		"filename": filename,
	})
}

