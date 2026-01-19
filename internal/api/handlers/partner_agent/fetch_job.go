package partner_agent

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"print-pro-backend/internal/middleware/auth_middleware"
	"print-pro-backend/internal/models/printjob"
	"strings"
)

// FetchJob handles requests from partner agent to fetch the next print job
// Queries database for pending jobs with status 'pending' for the authenticated partner
func (h *AgentHandler) FetchJob(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get user from context (set by OptionalAuthMiddleware if JWT is present)
	user, ok := auth_middleware.GetUserFromContext(r)
	if !ok {
		log.Printf("ERROR: User not found in context - partner agent must be authenticated")
		http.Error(w, "Unauthorized - partner agent must be authenticated", http.StatusUnauthorized)
		return
	}

	// Verify user is a partner
	if user.UserType != "partner" {
		log.Printf("ERROR: Non-partner user attempting to fetch jobs")
		http.Error(w, "Forbidden - only partners can fetch jobs", http.StatusForbidden)
		return
	}

	// Get partner_email from account_email (user.ID is the email)
	accountEmail := user.ID

	// Get partner profile to get partner_email
	// Note: We need to access partnerProfileRepo, but it's not in AgentHandler
	// For now, we'll use accountEmail as partnerEmail (they should be the same for partners)
	// If needed, we can add partnerProfileRepo to AgentHandler later
	partnerEmail := accountEmail

	// Query database for pending jobs for this partner
	if h.printJobRepo == nil {
		log.Printf("ERROR: PrintJobRepository is not available")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Get pending jobs from database
	pendingJobs, err := h.printJobRepo.GetPendingByPartnerEmail(ctx, partnerEmail)
	if err != nil {
		log.Printf("ERROR: Failed to get pending jobs from database: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// If no pending jobs, return 204 No Content
	if len(pendingJobs) == 0 {
		w.WriteHeader(http.StatusNoContent)
		log.Printf("INFO: No pending jobs available for partner - agent pinged but no pending jobs")
		return
	}

	// Get the first pending job (oldest first, as per GetPendingByPartnerEmail ordering)
	printJob := pendingJobs[0]
	fileName := printJob.Filename

	log.Printf("INFO: Agent pinged - Found pending job - File: %s, Job ID: %d", fileName, printJob.ID)

	// CRITICAL: Update database status to "processing" when file is sent to agent
	// This ensures partner dashboard shows correct status immediately
	if err := h.printJobRepo.UpdateStatus(ctx, fileName, "processing"); err != nil {
		log.Printf("WARNING: Failed to update print job status to 'processing': %v", err)
		http.Error(w, "Failed to update job status", http.StatusInternalServerError)
		return
	}
	log.Printf("INFO: Print job status updated to 'processing' - File: %s", fileName)

	// CRITICAL: Update database status to "processing" when file is sent to agent
	// This ensures partner dashboard shows correct status immediately
	if h.printJobRepo != nil {
		if err := h.printJobRepo.UpdateStatus(ctx, fileName, "processing"); err != nil {
			log.Printf("WARNING: Failed to update print job status to 'processing': %v", err)
			// Continue - file will still be sent to agent
		} else {
			log.Printf("INFO: Print job status updated to 'processing' - File: %s", fileName)
		}
	}

	// Read file from ready folder
	filePath := filepath.Join(h.readyDir, fileName)
	file, err := os.Open(filePath)
	if err != nil {
		log.Printf("ERROR: Failed to open file %s: %v", filePath, err)
		// Revert status back to pending if file is missing
		h.printJobRepo.UpdateStatus(ctx, fileName, "pending")
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}
	defer file.Close()

	// Get file info for content length
	fileInfo, err := file.Stat()
	if err != nil {
		log.Printf("ERROR: Failed to get file info for %s: %v", filePath, err)
		// Revert status back to pending if file read fails
		h.printJobRepo.UpdateStatus(ctx, fileName, "pending")
		http.Error(w, "Failed to read file", http.StatusInternalServerError)
		return
	}

	// Get print parameters from the print job we already retrieved
	var colorValue bool = false // Default to black & white
	var numCopiesValue int = 1   // Default to 1 copy
	var startPageValue *int = nil // Default to nil (print all pages from start)
	var endPageValue *int = nil   // Default to nil (print all pages to end)
	var pageFilterTypeValue *string = nil // Default to nil (agent will treat as "all")
	var individualColorPagesValue []int = nil // Default to nil (use global color setting)
	var skipPagesValue []int = nil // Default to nil (no pages to skip)
	var backToBackValue bool = false // Default to simplex (one side)
	var printTypeValue *string = nil // Default to nil (agent will use default/A4)
	var cropOptionsValue *printjob.CropOptions = nil // Default to nil (no cropping)
	var selectedPrinterName string = "" // Selected printer name for this job

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
	// Get print_type (prefer PrintType over PType)
	if printJob.PrintType != nil {
		printTypeValue = printJob.PrintType
	} else if printJob.PType != nil {
		printTypeValue = printJob.PType
	}
	// Get crop_options from page_options
	if printJob.PageOptions.CropOptions != nil {
		cropOptionsValue = printJob.PageOptions.CropOptions
	}
	log.Printf("INFO: Retrieved print parameters for '%s' - Color: %v, Copies: %d, StartPage: %v, EndPage: %v, PageFilterType: %v, IndividualColorPages: %v, SkipPages: %v, BackToBack: %v, PrintType: %v, CropOptions: %+v", 
		fileName, colorValue, numCopiesValue, startPageValue, endPageValue, pageFilterTypeValue, individualColorPagesValue, skipPagesValue, backToBackValue, printTypeValue, cropOptionsValue)

	// MULTI-PRINTER SUPPORT: Select appropriate printer for this job
	// Get synced printers from agent
	syncedPrinters := h.GetSyncedPrinters()
	if len(syncedPrinters) > 0 {
		// Smart printer selection: prefer color printer for color jobs, otherwise first available
		selectedPrinterName = h.selectPrinterForJob(syncedPrinters, colorValue, individualColorPagesValue)
		log.Printf("INFO: Selected printer '%s' for job '%s' (Color: %v, Available printers: %d)", selectedPrinterName, fileName, colorValue, len(syncedPrinters))
	} else {
		log.Printf("WARNING: No synced printers available - agent will use default printer")
	}

	// Set headers so the agent knows the filename and print parameters
	// Python script expects X-File-Name header
	w.Header().Set("X-File-Name", fileName)
	w.Header().Set("Content-Length", fmt.Sprintf("%d", fileInfo.Size()))
	
	// MULTI-PRINTER SUPPORT: Send selected printer name to agent
	if selectedPrinterName != "" {
		w.Header().Set("X-Print-Printer-Name", selectedPrinterName)
		log.Printf("INFO: Sending printer name '%s' to agent for job '%s'", selectedPrinterName, fileName)
	}
	
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
	
	// Send print_type (paper size)
	if printTypeValue != nil {
		w.Header().Set("X-Print-Type", *printTypeValue)
	}
	
	// Send crop_options (as JSON string)
	if cropOptionsValue != nil {
		cropOptionsJSON, err := json.Marshal(cropOptionsValue)
		if err == nil {
			w.Header().Set("X-Print-Crop-Options", string(cropOptionsJSON))
		} else {
			log.Printf("WARNING: Failed to marshal crop_options: %v", err)
		}
	}
	
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
		// Revert status back to pending if streaming fails
		h.printJobRepo.UpdateStatus(ctx, fileName, "pending")
		file.Close()
		return
	}
	file.Close()

	log.Printf("SUCCESS: File sent to agent - File: %s (status: processing, waiting for confirmation)", fileName)
}

// selectPrinterForJob intelligently selects a printer based on job requirements
// Returns the printer name (string) that should handle this job
func (h *AgentHandler) selectPrinterForJob(printers []map[string]interface{}, requiresColor bool, colorPages []int) string {
	if len(printers) == 0 {
		return ""
	}

	// If job requires color (either global color or individual color pages), prefer color-capable printers
	if requiresColor || (colorPages != nil && len(colorPages) > 0) {
		// Look for printers with "color" in name (case-insensitive) or status indicating color capability
		for _, printer := range printers {
			printerName := h.extractPrinterName(printer)
			if printerName != "" {
				// Check if printer name suggests color capability
				nameUpper := strings.ToUpper(printerName)
				if strings.Contains(nameUpper, "COLOR") || strings.Contains(nameUpper, "INKJET") || 
				   strings.Contains(nameUpper, "PIXMA") || strings.Contains(nameUpper, "DESKJET") {
					log.Printf("INFO: Selected color-capable printer '%s' for color job", printerName)
					return printerName
				}
			}
		}
	}

	// For non-color jobs or if no color printer found, use first available printer
	// Extract printer name from first printer (could be string or map)
	firstPrinterName := h.extractPrinterName(printers[0])
	if firstPrinterName != "" {
		log.Printf("INFO: Selected first available printer '%s'", firstPrinterName)
		return firstPrinterName
	}

	// Fallback: return empty string (agent will use default)
	log.Printf("WARNING: Could not extract printer name from synced printers, agent will use default")
	return ""
}

// extractPrinterName extracts printer name from various formats
// Supports: string, map[string]interface{} with "name" key
func (h *AgentHandler) extractPrinterName(printer interface{}) string {
	switch v := printer.(type) {
	case string:
		return v
	case map[string]interface{}:
		// Try "name" key first
		if name, ok := v["name"].(string); ok && name != "" {
			return name
		}
		// Try "printer_name" key
		if name, ok := v["printer_name"].(string); ok && name != "" {
			return name
		}
		// Try any string value in the map
		for _, val := range v {
			if str, ok := val.(string); ok && str != "" {
				return str
			}
		}
	}
	return ""
}
