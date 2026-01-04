package partner_agent

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
)

// ConfirmPrint handles requests from partner agent to confirm printing completion
// Updates database status to "completed" and removes filename from Redis processing queue
// File stays in ready folder (no file moves) for better performance and reprint capability
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

	// Update database status to "completed" (fast operation, no file moves)
	if h.printJobRepo != nil {
		if err := h.printJobRepo.UpdateStatus(ctx, req.Filename, "completed"); err != nil {
			log.Printf("WARNING: Failed to update print job status: %v", err)
			// Continue - Redis cleanup is more important
		} else {
			log.Printf("INFO: Print job status updated to 'completed' - File: %s", req.Filename)
		}
	}

	// Remove filename from processing queue using LREM (fast Redis operation)
	if h.redisClient != nil {
		removed, err := h.redisClient.LREM(ctx, processingQueueKey, 1, req.Filename)
		if err != nil {
			log.Printf("ERROR: Failed to remove filename from processing queue: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		if removed == 0 {
			log.Printf("WARNING: Filename not found in processing queue: %s", req.Filename)
			// Continue anyway - might have been removed already
		} else {
			log.Printf("INFO: Filename removed from processing queue - File: %s", req.Filename)
		}
	}

	// NOTE: File stays in ready folder - no file moves for better performance
	// File can be reprinted by clearing Redis key and updating status back to "pending"

	log.Printf("SUCCESS: Print confirmed - File: %s (status updated, Redis cleaned, file remains in ready folder)", req.Filename)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status":   "success",
		"message":  "Print confirmed successfully",
		"filename": req.Filename,
	})
}

