package partner_agent

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
)

// FetchJob handles requests from partner agent to fetch the next print job
// IMPORTANT: Agent ONLY fetches from processing folder (partner has explicitly queued/accepted)
// - Ready folder is ONLY for partner to see and queue (NOT sent to agent)
// - Processing folder = partner has accepted/queued → agent can fetch
// - After sending file to agent, it's immediately moved to archived folder
func (h *AgentHandler) FetchJob(w http.ResponseWriter, r *http.Request) {
	// Ensure processing directory exists
	if err := os.MkdirAll(h.processingDir, 0755); err != nil {
		log.Printf("ERROR: Failed to create processing directory: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// ONLY check processing folder (partner has explicitly queued/accepted)
	// DO NOT check ready folder - those are only for partner to see
	pdfFiles, _ := filepath.Glob(filepath.Join(h.processingDir, "*.pdf"))
	psFiles, _ := filepath.Glob(filepath.Join(h.processingDir, "*.ps"))
	files := append(pdfFiles, psFiles...)
	
	if len(files) == 0 {
		w.WriteHeader(http.StatusNoContent)
		log.Printf("INFO: No jobs available in processing folder - partner has not queued any files yet")
		return
	}

	// Sort to get the oldest file first (FIFO queue)
	sort.Strings(files)
	targetFile := files[0]
	fileName := filepath.Base(targetFile)

	log.Printf("INFO: Agent pinged - Found job in processing folder: %s (sending to agent)", fileName)

	// Open the file from processing directory
	file, err := os.Open(targetFile)
	if err != nil {
		log.Printf("ERROR: Failed to open file %s: %v", targetFile, err)
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}
	defer file.Close()

	// Get file info for content length
	fileInfo, err := file.Stat()
	if err != nil {
		log.Printf("ERROR: Failed to get file info for %s: %v", targetFile, err)
		http.Error(w, "Failed to read file", http.StatusInternalServerError)
		return
	}

	// Set headers so the agent knows the filename
	w.Header().Set("X-Job-Filename", fileName)
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
		file.Close()
		return
	}
	file.Close() // Close before moving
	
	// After successfully sending file to agent, move it to archived folder immediately
	// Ensure archive directory exists
	if err := os.MkdirAll(h.archiveDir, 0755); err != nil {
		log.Printf("ERROR: Failed to create archive directory: %v", err)
		// Don't fail the request, just log the error
	} else {
		archivePath := filepath.Join(h.archiveDir, fileName)
		if err := os.Rename(targetFile, archivePath); err != nil {
			log.Printf("ERROR: Failed to move file to archive after sending: %v", err)
			// Don't fail the request, just log the error
		} else {
			log.Printf("SUCCESS: File sent to agent and moved to archived folder - File: %s", fileName)
		}
	}
}

