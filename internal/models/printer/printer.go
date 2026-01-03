package printer

import "time"

// Printer represents a printer in the database
type Printer struct {
	ID          int64     `db:"id" json:"id"`
	PartnerID   int64     `db:"partner_id" json:"partner_id"`
	PrinterName string    `db:"printer_name" json:"printer_name"`
	SerialNumber string   `db:"serial_number" json:"serial_number"`
	Status      string    `db:"status" json:"status"` // online, offline, etc.
	LastSeen    time.Time `db:"last_seen" json:"last_seen"`
	CreatedAt   time.Time `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time `db:"updated_at" json:"updated_at"`
}

// CreatePrinterRequest represents the request to create a new printer
type CreatePrinterRequest struct {
	PartnerID   int64   `json:"partner_id" binding:"required"`
	ModelName   string  `json:"model_name" binding:"required"`
	IsColor     bool    `json:"is_color"`
	Status      string  `json:"status"`
	PricePerPage float64 `json:"price_per_page" binding:"required,min=0"`
}

// UpdatePrinterRequest represents the request to update a printer
type UpdatePrinterRequest struct {
	ModelName   string  `json:"model_name,omitempty"`
	IsColor     *bool   `json:"is_color,omitempty"`
	Status      string  `json:"status,omitempty"`
	PricePerPage *float64 `json:"price_per_page,omitempty" binding:"omitempty,min=0"`
}

// PrinterStatus constants
const (
	StatusAvailable   = "available"
	StatusBusy        = "busy"
	StatusMaintenance = "maintenance"
	StatusOffline     = "offline"
)

