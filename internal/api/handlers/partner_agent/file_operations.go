package partner_agent

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
)

// QueueJobForAgent queues a PDF file to the ready folder and pushes filename to Redis list
// This is called when a new PDF is uploaded
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

	// Push filename to Redis ready queue (only if not already there to avoid duplicates)
	if h.redisClient != nil {
		ctx := context.Background()
		queueKey := "printer:queue:ready"
		
		// Check if already in queue to avoid duplicates
		queueItems, err := h.redisClient.LRANGE(ctx, queueKey, 0, -1)
		alreadyInQueue := false
		if err == nil {
			for _, item := range queueItems {
				if item == fileName {
					alreadyInQueue = true
					break
				}
			}
		}
		
		if !alreadyInQueue {
			if err := h.redisClient.LPUSH(ctx, queueKey, fileName); err != nil {
				log.Printf("ERROR: Failed to push filename to Redis queue: %v", err)
				return fmt.Errorf("failed to queue file in Redis: %v", err)
			}
			log.Printf("INFO: Filename pushed to Redis ready queue - File: %s", fileName)
		} else {
			log.Printf("INFO: Filename already in Redis ready queue - File: %s", fileName)
		}
	} else {
		log.Printf("WARNING: Redis client is nil, cannot queue file to Redis")
	}

	log.Printf("INFO: Job queued to ready folder and Redis - File: %s", fileName)
	return nil
}

// MoveToProcessing ensures filename is in Redis ready queue (when partner clicks print)
// The file stays in ready folder, Redis tracks the queue state
// This ensures the file is available for agent to fetch via RPOPLPUSH
func (h *AgentHandler) MoveToProcessing(filename string) error {
	// Check if file exists in ready folder
	readyPath := filepath.Join(h.readyDir, filename)
	if _, err := os.Stat(readyPath); os.IsNotExist(err) {
		return fmt.Errorf("file not found in ready folder: %s", filename)
	}

	// Ensure filename is in Redis ready queue (check first to avoid duplicates)
	if h.redisClient == nil {
		return fmt.Errorf("Redis client is not available - cannot queue file")
	}

	ctx := context.Background()
	queueKey := "printer:queue:ready"
	
	// Check if filename is already in the queue
	queueItems, err := h.redisClient.LRANGE(ctx, queueKey, 0, -1)
	if err != nil {
		log.Printf("WARNING: Failed to check Redis queue, will attempt to push anyway: %v", err)
		// Continue to try pushing - might be a transient error
	} else {
		// Check if filename already exists in queue
		for _, item := range queueItems {
			if item == filename {
				log.Printf("INFO: Filename already in Redis ready queue - File: %s", filename)
				return nil // Already queued, success
			}
		}
	}
	
	// Push filename to Redis ready queue (not already there)
	if err := h.redisClient.LPUSH(ctx, queueKey, filename); err != nil {
		log.Printf("ERROR: Failed to push filename to Redis queue: %v", err)
		return fmt.Errorf("failed to queue file in Redis: %v", err)
	}
	
	log.Printf("INFO: Filename successfully pushed to Redis ready queue - File: %s", filename)
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

// GetReadyQueue returns all filenames in the ready queue from Redis
func (h *AgentHandler) GetReadyQueue(ctx context.Context) ([]string, error) {
	if h.redisClient == nil {
		return []string{}, nil
	}
	queueKey := "printer:queue:ready"
	return h.redisClient.LRANGE(ctx, queueKey, 0, -1)
}

// GetProcessingQueue returns all filenames in the processing queue from Redis
func (h *AgentHandler) GetProcessingQueue(ctx context.Context) ([]string, error) {
	if h.redisClient == nil {
		return []string{}, nil
	}
	queueKey := "printer:queue:processing"
	return h.redisClient.LRANGE(ctx, queueKey, 0, -1)
}

