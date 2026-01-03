package partner_agent

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

// FetchJob handles requests from partner agent to fetch the next print job
// Uses Redis RPOPLPUSH to atomically move a job from ready queue to processing queue
// This ensures only one agent gets a job even if multiple agents ping simultaneously
func (h *AgentHandler) FetchJob(w http.ResponseWriter, r *http.Request) {
	if h.redisClient == nil {
		log.Printf("ERROR: Redis client is not available")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	ctx := context.Background()
	readyQueueKey := "printer:queue:ready"
	processingQueueKey := "printer:queue:processing"

	// Atomically move one job from ready queue to processing queue
	// RPOPLPUSH ensures atomic operation - only one agent gets the job
	fileName, err := h.redisClient.RPOPLPUSH(ctx, readyQueueKey, processingQueueKey)
	if err != nil {
		log.Printf("ERROR: Failed to pop job from Redis queue: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// If fileName is empty, no jobs available
	if fileName == "" {
		w.WriteHeader(http.StatusNoContent)
		log.Printf("INFO: No jobs available in Redis ready queue - agent pinged but queue is empty")
		return
	}

	log.Printf("INFO: Agent pinged - Job atomically moved from ready to processing queue - File: %s", fileName)

	// File is still in ready folder (we don't move it physically anymore)
	// Read file from ready folder
	filePath := filepath.Join(h.readyDir, fileName)
	file, err := os.Open(filePath)
	if err != nil {
		log.Printf("ERROR: Failed to open file %s: %v", filePath, err)
		// Job is already in processing queue, but file is missing
		// Remove it from processing queue to prevent it from being stuck
		h.redisClient.LREM(ctx, processingQueueKey, 1, fileName)
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}
	defer file.Close()

	// Get file info for content length
	fileInfo, err := file.Stat()
	if err != nil {
		log.Printf("ERROR: Failed to get file info for %s: %v", filePath, err)
		// Remove from processing queue
		h.redisClient.LREM(ctx, processingQueueKey, 1, fileName)
		http.Error(w, "Failed to read file", http.StatusInternalServerError)
		return
	}

	// Set headers so the agent knows the filename
	// Python script expects X-File-Name header
	w.Header().Set("X-File-Name", fileName)
	w.Header().Set("Content-Length", fmt.Sprintf("%d", fileInfo.Size()))
	
	// Set content type based on file extension
	if filepath.Ext(fileName) == ".ps" {
		w.Header().Set("Content-Type", "application/postscript")
	} else {
		w.Header().Set("Content-Type", "application/pdf")
	}
	
	// Stream the file directly to the agent
	_, err = io.Copy(w, file)
	if err != nil {
		log.Printf("ERROR: Failed to stream file %s: %v", fileName, err)
		// Note: Job is already in processing queue, will be cleaned up on confirm
		file.Close()
		return
	}
	file.Close()

	log.Printf("SUCCESS: File sent to agent - File: %s (job in processing queue, waiting for confirmation)", fileName)
}

