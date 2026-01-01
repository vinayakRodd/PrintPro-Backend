package partner_agent

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

// ConfirmPrint handles requests from partner agent to confirm printing completion
func (h *AgentHandler) ConfirmPrint(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ConfirmRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("ERROR: Failed to decode confirm request: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Filename == "" {
		http.Error(w, "Filename is required", http.StatusBadRequest)
		return
	}

	// File should already be in archived folder (moved there during fetch)
	// But check processing and ready folders for backward compatibility
	oldPath := filepath.Join(h.archiveDir, req.Filename)
	if _, err := os.Stat(oldPath); os.IsNotExist(err) {
		// Try processing folder
		oldPath = filepath.Join(h.processingDir, req.Filename)
		if _, err := os.Stat(oldPath); os.IsNotExist(err) {
			// Try ready folder
			oldPath = filepath.Join(h.readyDir, req.Filename)
		}
	}
	newPath := filepath.Join(h.archiveDir, req.Filename)

	// Check if file exists
	if _, err := os.Stat(oldPath); os.IsNotExist(err) {
		log.Printf("WARNING: File not found for archiving: %s", req.Filename)
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	// Ensure archive directory exists
	if err := os.MkdirAll(h.archiveDir, 0755); err != nil {
		log.Printf("ERROR: Failed to create archive directory: %v", err)
		http.Error(w, "Failed to create archive directory", http.StatusInternalServerError)
		return
	}

	// Move file to archive (if not already there)
	if oldPath != newPath {
		err := os.Rename(oldPath, newPath)
		if err != nil {
			log.Printf("ERROR: Failed to archive %s: %v", req.Filename, err)
			http.Error(w, "Failed to archive file", http.StatusInternalServerError)
			return
		}
	}

	log.Printf("INFO: Successfully confirmed and archived: %s", req.Filename)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status":   "success",
		"message":  "File archived successfully",
		"filename": req.Filename,
	})
}

