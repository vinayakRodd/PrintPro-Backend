package print_handler

import (
	"print-pro-backend/internal/api/handlers/partner_agent"
	"print-pro-backend/internal/api/handlers/websocket"
	"print-pro-backend/internal/models/jobcost"
	"print-pro-backend/internal/models/printjob"
	"print-pro-backend/internal/repositories"
)

// PrintHandler handles print requests from partners
type PrintHandler struct {
	uploadDir              string
	agentHandler           *partner_agent.AgentHandler
	partnerProfileRepo     *repositories.PartnerProfileRepository
	printJobRepo           *repositories.PrintJobRepository
	jobCostRepo            *repositories.JobCostRepository
	accountRepository      *repositories.AccountRepository
	costCalculator         interface {
		CalculateCost(filePath string, pageOptions printjob.PageOptions, color *bool, numCopies *int, individualColorPages []int) (*jobcost.JobCost, error)
	}
	wsHub                  *websocket.Hub
}

// NewPrintHandler creates a new print handler
func NewPrintHandler(
	uploadDir string,
	agentHandler *partner_agent.AgentHandler,
	partnerProfileRepo *repositories.PartnerProfileRepository,
	printJobRepo *repositories.PrintJobRepository,
	jobCostRepo *repositories.JobCostRepository,
	accountRepository *repositories.AccountRepository,
	costCalculator interface {
		CalculateCost(filePath string, pageOptions printjob.PageOptions, color *bool, numCopies *int, individualColorPages []int) (*jobcost.JobCost, error)
	},
	wsHub *websocket.Hub,
) *PrintHandler {
	return &PrintHandler{
		uploadDir:          uploadDir,
		agentHandler:       agentHandler,
		partnerProfileRepo: partnerProfileRepo,
		printJobRepo:       printJobRepo,
		jobCostRepo:        jobCostRepo,
		accountRepository:  accountRepository,
		costCalculator:     costCalculator,
		wsHub:              wsHub,
	}
}

// PrintRequest represents the request to print a file
type PrintRequest struct {
	Filename string `json:"filename"`
	Printer  string `json:"printer,omitempty"` // Optional: specific printer name (if not provided, uses default)
}

