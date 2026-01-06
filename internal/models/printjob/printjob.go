package printjob

import "time"

// PrintJob represents a print job in the database
type PrintJob struct {
	ID        int64     `db:"id" json:"id"`
	AccountID *int64    `db:"account_id" json:"account_id,omitempty"`     // Nullable (customer account ID)
	PartnerID int64     `db:"partner_id" json:"partner_id"`              // Required - partner account ID
	PrinterID *int64    `db:"printer_id" json:"printer_id,omitempty"`    // Nullable (assigned later)
	Filename  string    `db:"filename" json:"filename"`
	FileURL   string    `db:"file_url" json:"file_url"`
	PType     *string   `db:"p_type" json:"p_type,omitempty"`            // Nullable - print type (e.g., "A4", "A3", etc.)
	Color     *bool     `db:"color" json:"color,omitempty"`              // Nullable - color printing (default: false)
	NumCopies *int      `db:"num_copies" json:"num_copies,omitempty"`    // Nullable - number of copies (default: 1)
	StartPage *int      `db:"start_page" json:"start_page,omitempty"`    // Nullable - starting page number (1-indexed, NULL = from first page)
	EndPage   *int      `db:"end_page" json:"end_page,omitempty"`        // Nullable - ending page number (1-indexed, NULL = to last page)
	PageFilterType *string `db:"page_filter_type" json:"page_filter_type,omitempty"` // Optional page filter: "all" (default), "odd", "even"
	Status    *string   `db:"status" json:"status,omitempty"`            // Nullable - pending, processing, completed, failed, cancelled
	TotalCost *string   `db:"total_cost" json:"total_cost,omitempty"`    // Nullable (calculated later) - PostgreSQL numeric type
	CreatedAt *time.Time `db:"created_at" json:"created_at,omitempty"`   // Nullable
	UpdatedAt *time.Time `db:"updated_at" json:"updated_at,omitempty"`  // Nullable
}

// CreatePrintJobRequest represents the request to create a new print job
type CreatePrintJobRequest struct {
	AccountID *int64 `json:"account_id,omitempty"` // Optional - customer account ID
	PartnerID int64  `json:"partner_id" binding:"required"`
	PrinterID *int64 `json:"printer_id,omitempty"` // Optional (can be assigned later)
	Filename  string `json:"filename" binding:"required"`
	FileURL   string `json:"file_url" binding:"required,url"`
}

// UpdatePrintJobRequest represents the request to update a print job
type UpdatePrintJobRequest struct {
	PrinterID *int64  `json:"printer_id,omitempty"`
	Status    string  `json:"status,omitempty"`
	TotalCost *string `json:"total_cost,omitempty"` // PostgreSQL numeric type
}

// PrintJobStatus constants
const (
	StatusPending    = "pending"
	StatusProcessing = "processing"
	StatusCompleted  = "completed"
	StatusFailed     = "failed"
	StatusCancelled  = "cancelled"
)

