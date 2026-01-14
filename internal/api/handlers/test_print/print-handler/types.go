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

// JobCostDTO represents the job cost data transfer object for API responses
// This DTO hides the internal database schema structure
type JobCostDTO struct {
	JobID          int64   `json:"job_id"`           // print_job_id (renamed for security)
	CustomerID     string  `json:"customer_id"`      // customer_email (renamed for security)
	PageInfo       PageInfoDTO `json:"page_info"`    // Grouped page information
	Pricing        PricingDTO  `json:"pricing"`      // Grouped pricing information
	Timestamp      TimestampDTO `json:"timestamp"`   // Grouped timestamp information
}

// PageInfoDTO groups page-related information
type PageInfoDTO struct {
	TotalPages      int `json:"total_pages"`
	PagesToPrint    int `json:"pages_to_print"`
	ColorPages      int `json:"color_pages"`
	BlackWhitePages int `json:"black_white_pages"`
}

// PricingDTO groups pricing-related information
type PricingDTO struct {
	NumCopies int     `json:"copies"`
	TotalCost float64 `json:"amount"` // total_cost (renamed)
	Currency  string  `json:"currency"`
}

// TimestampDTO groups timestamp information
type TimestampDTO struct {
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// JobCostsResponse represents the API response for job costs
type JobCostsResponse struct {
	Success bool         `json:"success"`
	Message string       `json:"message"`
	Data    []JobCostDTO `json:"data"`
	Count   int          `json:"count"`
}

