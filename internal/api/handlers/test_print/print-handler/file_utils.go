package print_handler

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// findMostRecentPDF finds the most recently modified PDF file in the upload directory
func (h *PrintHandler) findMostRecentPDF() (string, error) {
	// Read directory
	files, err := os.ReadDir(h.uploadDir)
	if err != nil {
		return "", fmt.Errorf("failed to read upload directory: %v", err)
	}

	var mostRecentPDF string
	var mostRecentTime int64 = 0

	// Find the most recent PDF file
	for _, file := range files {
		if file.IsDir() {
			continue
		}

		// Check if it's a PDF
		if strings.ToLower(filepath.Ext(file.Name())) != ".pdf" {
			continue
		}

		// Get file info
		info, err := file.Info()
		if err != nil {
			continue
		}

		// Check if this is the most recent
		modTime := info.ModTime().Unix()
		if modTime > mostRecentTime {
			mostRecentTime = modTime
			mostRecentPDF = file.Name()
		}
	}

	if mostRecentPDF == "" {
		return "", fmt.Errorf("no PDF files found in upload directory")
	}

	// Return absolute path
	pdfPath := filepath.Join(h.uploadDir, mostRecentPDF)
	absPath, err := filepath.Abs(pdfPath)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path: %v", err)
	}

	return absPath, nil
}

