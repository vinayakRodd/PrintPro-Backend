package printer

// Printer represents a printer in the database
type Printer struct {
	ID          int     `db:"id" json:"id"`
	PartnerID   int     `db:"partner_id" json:"partner_id"`
	ModelName   string  `db:"model_name" json:"model_name"`
	IsColor     bool    `db:"is_color" json:"is_color"`
	Status      string  `db:"status" json:"status"` // available, busy, maintenance, offline
	PricePerPage float64 `db:"price_per_page" json:"price_per_page"`
}

// CreatePrinterRequest represents the request to create a new printer
type CreatePrinterRequest struct {
	PartnerID   int     `json:"partner_id" binding:"required"`
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

