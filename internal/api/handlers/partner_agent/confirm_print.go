package partner_agent

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// ConfirmPrint handles requests from partner agent to confirm printing completion
// Removes filename from Redis processing queue and moves physical file to archived folder
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

	ctx := context.Background()
	processingQueueKey := "printer:queue:processing"

	// Remove filename from processing queue using LREM
	// count = 1 means remove first occurrence from head to tail
	if h.redisClient != nil {
		removed, err := h.redisClient.LREM(ctx, processingQueueKey, 1, req.Filename)
		if err != nil {
			log.Printf("ERROR: Failed to remove filename from processing queue: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		if removed == 0 {
			log.Printf("WARNING: Filename not found in processing queue: %s", req.Filename)
			// Continue anyway - file might have been removed already
		} else {
			log.Printf("INFO: Filename removed from processing queue - File: %s", req.Filename)
		}

		// Optionally: Log completion in Redis Set with TTL (24 hours)
		doneSetKey := "printer:done_today"
		if err := h.redisClient.SADD(ctx, doneSetKey, req.Filename); err == nil {
			// Set TTL on the set key (24 hours)
			h.redisClient.Set(ctx, doneSetKey+":ttl", "1", 24*time.Hour)
			log.Printf("INFO: Print completion logged in Redis set - File: %s", req.Filename)
		}
	}

	// Move physical file from ready folder to archived folder
	readyPath := filepath.Join(h.readyDir, req.Filename)
	archivePath := filepath.Join(h.archiveDir, req.Filename)

	// Check if file exists in ready folder
	if _, err := os.Stat(readyPath); os.IsNotExist(err) {
		log.Printf("WARNING: File not found in ready folder: %s (may have been archived already)", req.Filename)
		// Don't fail - Redis queue is already cleaned up
	} else {
		// Ensure archive directory exists
		if err := os.MkdirAll(h.archiveDir, 0755); err != nil {
			log.Printf("ERROR: Failed to create archive directory: %v", err)
			http.Error(w, "Failed to create archive directory", http.StatusInternalServerError)
			return
		}

		// Move file to archive
		if err := os.Rename(readyPath, archivePath); err != nil {
			log.Printf("ERROR: Failed to archive %s: %v", req.Filename, err)
			http.Error(w, "Failed to archive file", http.StatusInternalServerError)
			return
		}
		log.Printf("INFO: File moved to archived folder - File: %s", req.Filename)
	}

	log.Printf("SUCCESS: Print confirmed and job cleaned up - File: %s", req.Filename)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status":   "success",
		"message":  "File archived successfully",
		"filename": req.Filename,
	})
}

