package partner_agent

import (
	"context"
	"path/filepath"
	"sync"
	"print-pro-backend/internal/infrastructure"
	"print-pro-backend/internal/models/printjob"
)

// ConfirmRequest represents the request to confirm a print job
type ConfirmRequest struct {
	Filename string `json:"filename"`
	Status   string `json:"status,omitempty"` // Optional status field from Python script
}

// SyncPrintersRequest represents the request to sync printers from partner agent
// Supports both formats:
// 1. Array of strings: {"printers": ["Printer1", "Printer2"]}
// 2. Array of objects: {"printers": [{"name": "Printer1", "status": "online"}]}
type SyncPrintersRequest struct {
	Printers interface{} `json:"printers"` // Can be []string or []map[string]interface{}
}

// PrintJobRepository interface for print job operations
type PrintJobRepository interface {
	GetByFilename(ctx context.Context, filename string) (*printjob.PrintJob, error)
	UpdateStatus(ctx context.Context, filename, status string) error
}

// AgentHandler handles requests from partner agent
type AgentHandler struct {
	readyDir       string  // Only files explicitly requested for printing go here
	archiveDir     string
	processingDir string
	redisClient    *infrastructure.RedisClient
	syncedPrinters []map[string]interface{}
	printerMutex   sync.RWMutex
	printJobRepo   PrintJobRepository
}

// NewAgentHandler creates a new agent handler
func NewAgentHandler(baseDir, archiveDir string, redisClient *infrastructure.RedisClient, printJobRepo PrintJobRepository) *AgentHandler {
	readyDir := filepath.Join(baseDir, "ready")
	processingDir := filepath.Join(baseDir, "processing")
	return &AgentHandler{
		readyDir:       readyDir,
		archiveDir:     archiveDir,
		processingDir: processingDir,
		redisClient:    redisClient,
		syncedPrinters: make([]map[string]interface{}, 0),
		printJobRepo:   printJobRepo,
	}
}
