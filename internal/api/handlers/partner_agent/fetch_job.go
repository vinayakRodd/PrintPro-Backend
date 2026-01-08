package partner_agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
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

	// OPTIMIZATION: Get print parameters from database (can be slow, but happens after file is ready to stream)
	// This runs in parallel with file opening, so minimal impact
	var colorValue bool = false // Default to black & white
	var numCopiesValue int = 1   // Default to 1 copy
	var startPageValue *int = nil // Default to nil (print all pages from start)
	var endPageValue *int = nil   // Default to nil (print all pages to end)
	var pageFilterTypeValue *string = nil // Default to nil (agent will treat as "all")
	var individualColorPagesValue []int = nil // Default to nil (use global color setting)
	var skipPagesValue []int = nil // Default to nil (no pages to skip)
	var backToBackValue bool = false // Default to simplex (one side)
	
	if h.printJobRepo != nil {
		printJob, err := h.printJobRepo.GetByFilename(ctx, fileName)
		if err != nil {
			log.Printf("WARNING: Failed to get print job for filename '%s': %v (using defaults: color=false, copies=1, all pages)", fileName, err)
		} else {
			// Use values from database, with defaults if nil
			if printJob.Color != nil {
				colorValue = *printJob.Color
			}
			if printJob.NumCopies != nil {
				numCopiesValue = *printJob.NumCopies
			}
			// Extract page options from PageOptions structure
			if printJob.PageOptions.StartPage != nil {
				startPageValue = printJob.PageOptions.StartPage
			}
			if printJob.PageOptions.EndPage != nil {
				endPageValue = printJob.PageOptions.EndPage
			}
			if printJob.PageOptions.FilterType != nil {
				pageFilterTypeValue = printJob.PageOptions.FilterType
			}
			if printJob.PageOptions.ColorPages != nil && len(printJob.PageOptions.ColorPages) > 0 {
				individualColorPagesValue = printJob.PageOptions.ColorPages
			}
			if printJob.PageOptions.SkipPages != nil && len(printJob.PageOptions.SkipPages) > 0 {
				skipPagesValue = printJob.PageOptions.SkipPages
			}
			if printJob.BackToBack != nil {
				backToBackValue = *printJob.BackToBack
			}
			log.Printf("INFO: Retrieved print parameters for '%s' - Color: %v, Copies: %d, StartPage: %v, EndPage: %v, PageFilterType: %v, IndividualColorPages: %v, SkipPages: %v, BackToBack: %v", 
				fileName, colorValue, numCopiesValue, startPageValue, endPageValue, pageFilterTypeValue, individualColorPagesValue, skipPagesValue, backToBackValue)
		}
	} else {
		log.Printf("WARNING: PrintJobRepository not available, using defaults (color=false, copies=1, all pages)")
	}

	// Set headers so the agent knows the filename and print parameters
	// Python script expects X-File-Name header
	w.Header().Set("X-File-Name", fileName)
	w.Header().Set("Content-Length", fmt.Sprintf("%d", fileInfo.Size()))
	
	// Send print parameters as headers for partner agent
	w.Header().Set("X-Print-Color", fmt.Sprintf("%v", colorValue))      // "true" or "false"
	w.Header().Set("X-Print-Copies", fmt.Sprintf("%d", numCopiesValue)) // Number of copies
	
	// Send page range parameters (only if specified)
	if startPageValue != nil {
		w.Header().Set("X-Print-Start-Page", fmt.Sprintf("%d", *startPageValue))
	}
	if endPageValue != nil {
		w.Header().Set("X-Print-End-Page", fmt.Sprintf("%d", *endPageValue))
	}
	if pageFilterTypeValue != nil {
		w.Header().Set("X-Print-Page-Filter", fmt.Sprintf("%s", *pageFilterTypeValue))
	}
	if individualColorPagesValue != nil && len(individualColorPagesValue) > 0 {
		// Convert array to JSON string for header
		individualColorPagesJSON, err := json.Marshal(individualColorPagesValue)
		if err == nil {
			w.Header().Set("X-Print-Individual-Color-Pages", string(individualColorPagesJSON))
		}
	}
	if skipPagesValue != nil && len(skipPagesValue) > 0 {
		// Convert array to JSON string for header
		skipPagesJSON, err := json.Marshal(skipPagesValue)
		if err == nil {
			w.Header().Set("X-Print-Skip-Pages", string(skipPagesJSON))
		}
	}
	// Send back_to_back parameter
	w.Header().Set("X-Print-Back-To-Back", fmt.Sprintf("%v", backToBackValue))
	
	// Set content type based on file extension
	ext := strings.ToLower(filepath.Ext(fileName))
	switch ext {
	case ".ps":
		w.Header().Set("Content-Type", "application/postscript")
	case ".png":
		w.Header().Set("Content-Type", "image/png")
	case ".jpg", ".jpeg":
		w.Header().Set("Content-Type", "image/jpeg")
	default:
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

