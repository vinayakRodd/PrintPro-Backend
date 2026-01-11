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
// If the file is already in the ready folder, it only pushes to Redis (no copy)
func (h *AgentHandler) QueueJobForAgent(sourceFilePath string) error {
	// Ensure ready directory exists
	if err := os.MkdirAll(h.readyDir, 0755); err != nil {
		return fmt.Errorf("failed to create ready directory: %v", err)
	}

	fileName := filepath.Base(sourceFilePath)
	targetPath := filepath.Join(h.readyDir, fileName)

	// Check if file is already in ready folder (same path)
	// If so, skip copying and just push to Redis
	if sourceFilePath == targetPath {
		log.Printf("INFO: File is already in ready folder, skipping copy - File: %s", fileName)
	} else {
		// File is not in ready folder, copy it there
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
		log.Printf("INFO: File copied to ready folder - File: %s", fileName)
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
	// OPTIMIZATION: Skip file existence check - already verified in PrintFile handler
	// This saves a file I/O operation

	// Ensure filename is in Redis ready queue
	if h.redisClient == nil {
		log.Printf("ERROR: Redis client is nil - cannot queue file: %s", filename)
		return fmt.Errorf("Redis client is not available - cannot queue file")
	}

	ctx := context.Background()
	queueKey := "printer:queue:ready"
	
	// OPTIMIZATION: Just push to Redis - no duplicate check (saves LRANGE operation)
	// Redis list can have duplicates, and agent will handle it via RPOPLPUSH
	// Use LPUSH to add to head - RPOPLPUSH removes from tail, so this creates FIFO queue (oldest first)
	if err := h.redisClient.LPUSH(ctx, queueKey, filename); err != nil {
		log.Printf("ERROR: Failed to push filename to Redis queue '%s': %v - File: %s", queueKey, err, filename)
		return fmt.Errorf("failed to queue file in Redis: %v", err)
	}
	
	// Verify the file was added to the queue (helps debug Redis connection issues)
	queueLength, err := h.redisClient.LLEN(ctx, queueKey)
	if err != nil {
		log.Printf("WARNING: Failed to verify queue length after adding file: %v", err)
	} else {
		log.Printf("INFO: Filename successfully pushed to Redis ready queue - File: %s, Queue length: %d", filename, queueLength)
	}
	
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

