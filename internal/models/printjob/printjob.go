package printjob

import "time"

// PrintJob represents a print job in the database
type PrintJob struct {
	ID        int64     `db:"id" json:"id"`
	UserID    *int64    `db:"user_id" json:"user_id,omitempty"`     // Nullable (guest printing)
	PartnerID int64     `db:"partner_id" json:"partner_id"`
	PrinterID *int64    `db:"printer_id" json:"printer_id,omitempty"` // Nullable (assigned later)
	Filename  string    `db:"filename" json:"filename"`
	FileURL   string    `db:"file_url" json:"file_url"`
	Status    string    `db:"status" json:"status"` // pending, processing, completed, failed, cancelled
	TotalCost *float64  `db:"total_cost" json:"total_cost,omitempty"` // Nullable (calculated later)
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

// CreatePrintJobRequest represents the request to create a new print job
type CreatePrintJobRequest struct {
	UserID    *int64 `json:"user_id,omitempty"` // Optional for guest printing
	PartnerID int64  `json:"partner_id" binding:"required"`
	PrinterID *int64 `json:"printer_id,omitempty"` // Optional (can be assigned later)
	Filename  string `json:"filename" binding:"required"`
	FileURL   string `json:"file_url" binding:"required,url"`
}

// UpdatePrintJobRequest represents the request to update a print job
type UpdatePrintJobRequest struct {
	PrinterID *int64  `json:"printer_id,omitempty"`
	Status    string  `json:"status,omitempty"`
	TotalCost *float64 `json:"total_cost,omitempty" binding:"omitempty,min=0"`
}

// PrintJobStatus constants
const (
	StatusPending    = "pending"
	StatusProcessing = "processing"
	StatusCompleted  = "completed"
	StatusFailed     = "failed"
	StatusCancelled  = "cancelled"
)

