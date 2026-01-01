package partner_agent

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
)

// QueueJobForAgent queues a PDF file to the ready folder (only called when frontend explicitly requests print)
func (h *AgentHandler) QueueJobForAgent(sourceFilePath string) error {
	// Ensure ready directory exists
	if err := os.MkdirAll(h.readyDir, 0755); err != nil {
		return fmt.Errorf("failed to create ready directory: %v", err)
	}

	fileName := filepath.Base(sourceFilePath)
	targetPath := filepath.Join(h.readyDir, fileName)

	// Open source file
	sourceFile, err := os.Open(sourceFilePath)
	if err != nil {
		return fmt.Errorf("failed to open source file: %v", err)
	}
	defer sourceFile.Close()

	// Create destination file in ready folder
	destFile, err := os.Create(targetPath)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %v", err)
	}
	defer destFile.Close()

	// Copy file content to ready folder
	_, err = io.Copy(destFile, sourceFile)
	if err != nil {
		os.Remove(targetPath) // Clean up on error
		return fmt.Errorf("failed to copy file: %v", err)
	}

	log.Printf("INFO: Job queued to ready folder for agent - File: %s", fileName)
	return nil
}

// MoveToProcessing moves a file from ready folder to processing folder (when frontend queues it)
func (h *AgentHandler) MoveToProcessing(filename string) error {
	// Ensure directories exist
	if err := os.MkdirAll(h.processingDir, 0755); err != nil {
		return fmt.Errorf("failed to create processing directory: %v", err)
	}

	readyPath := filepath.Join(h.readyDir, filename)
	processingPath := filepath.Join(h.processingDir, filename)

	// Check if file exists in ready folder
	if _, err := os.Stat(readyPath); os.IsNotExist(err) {
		return fmt.Errorf("file not found in ready folder: %s", filename)
	}

	// Move file from ready to processing
	if err := os.Rename(readyPath, processingPath); err != nil {
		return fmt.Errorf("failed to move file to processing: %v", err)
	}

	log.Printf("INFO: File moved to processing folder - File: %s", filename)
	return nil
}

// QueueJobToProcessing queues a PDF file directly to the processing folder (when partner requests print)
func (h *AgentHandler) QueueJobToProcessing(sourceFilePath string) error {
	// Ensure processing directory exists
	if err := os.MkdirAll(h.processingDir, 0755); err != nil {
		return fmt.Errorf("failed to create processing directory: %v", err)
	}

	fileName := filepath.Base(sourceFilePath)
	targetPath := filepath.Join(h.processingDir, fileName)

	// Open source file
	sourceFile, err := os.Open(sourceFilePath)
	if err != nil {
		return fmt.Errorf("failed to open source file: %v", err)
	}
	defer sourceFile.Close()

	// Create destination file in processing folder
	destFile, err := os.Create(targetPath)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %v", err)
	}
	defer destFile.Close()

	// Copy file content to processing folder
	_, err = io.Copy(destFile, sourceFile)
	if err != nil {
		os.Remove(targetPath) // Clean up on error
		return fmt.Errorf("failed to copy file: %v", err)
	}

	log.Printf("INFO: Job queued directly to processing folder - File: %s", fileName)
	return nil
}

// GetReadyDir returns the ready directory path
func (h *AgentHandler) GetReadyDir() string {
	return h.readyDir
}

// GetProcessingDir returns the processing directory path
func (h *AgentHandler) GetProcessingDir() string {
	return h.processingDir
}

