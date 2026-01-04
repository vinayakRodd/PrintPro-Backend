package print_handler

import (
	"print-pro-backend/internal/api/handlers/partner_agent"
	"print-pro-backend/internal/repositories"
)

// PrintHandler handles print requests from partners
type PrintHandler struct {
	uploadDir              string
	agentHandler           *partner_agent.AgentHandler
	partnerProfileRepo     *repositories.PartnerProfileRepository
	printJobRepo           *repositories.PrintJobRepository
}

// NewPrintHandler creates a new print handler
func NewPrintHandler(uploadDir string, agentHandler *partner_agent.AgentHandler, partnerProfileRepo *repositories.PartnerProfileRepository, printJobRepo *repositories.PrintJobRepository) *PrintHandler {
	return &PrintHandler{
		uploadDir:          uploadDir,
		agentHandler:       agentHandler,
		partnerProfileRepo: partnerProfileRepo,
		printJobRepo:       printJobRepo,
	}
}

// PrintRequest represents the request to print a file
type PrintRequest struct {
	Filename string `json:"filename"`
	Printer  string `json:"printer,omitempty"` // Optional: specific printer name (if not provided, uses default)
}

