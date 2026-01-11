package partner_agent

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"print-pro-backend/internal/models/printjob"
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

	// Get print job details before updating status (needed for cost calculation)
	var printJob *printjob.PrintJob
	if h.printJobRepo != nil {
		var err error
		printJob, err = h.printJobRepo.GetByFilename(ctx, req.Filename)
		if err != nil {
			log.Printf("WARNING: Failed to get print job for cost calculation: %v", err)
		}
	}

	// Update database status to "completed" (fast operation, no file moves)
	if h.printJobRepo != nil {
		if err := h.printJobRepo.UpdateStatus(ctx, req.Filename, "completed"); err != nil {
			log.Printf("WARNING: Failed to update print job status: %v", err)
			// Continue - Redis cleanup is more important
		} else {
			log.Printf("INFO: Print job status updated to 'completed' - File: %s", req.Filename)
		}
	}

	// Calculate and store cost now that job is completed
	if printJob != nil && h.costCalculator != nil && h.jobCostRepo != nil {
		// Get file path
		filePath := filepath.Join(h.readyDir, req.Filename)
		
		// Check if file exists
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			log.Printf("WARNING: File not found for cost calculation: %s", filePath)
		} else {
			// Use print job's page options
			pageOpts := printJob.PageOptions
			
			// Get individual color pages from page options
			individualColorPages := []int{}
			if len(pageOpts.ColorPages) > 0 {
				individualColorPages = pageOpts.ColorPages
			}
			
			// Calculate cost
			jobCost, err := h.costCalculator.CalculateCost(
				filePath,
				pageOpts,
				printJob.Color,
				printJob.NumCopies,
				individualColorPages,
			)
			
			if err != nil {
				log.Printf("WARNING: Failed to calculate cost for completed print job - %v", err)
			} else {
				// Store cost in job_cost table
				err = h.jobCostRepo.CreateOrUpdate(ctx, printJob.ID, jobCost)
				if err != nil {
					log.Printf("WARNING: Failed to store job cost - %v", err)
				} else {
					// Update total_cost in print_jobs table
					err = h.jobCostRepo.UpdateTotalCostInPrintJob(ctx, printJob.ID, jobCost.TotalCost)
					if err != nil {
						log.Printf("WARNING: Failed to update total_cost in print_jobs - %v", err)
					} else {
						log.Printf("SUCCESS: Cost calculated and stored for completed job - Total: ₹%.2f (Color: %d pages, B&W: %d pages, Copies: %d)", 
							jobCost.TotalCost, jobCost.ColorPages, jobCost.BlackWhitePages, jobCost.NumCopies)
					}
				}
			}
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

