package printjob

import (
	"encoding/json"
	"time"
)

// PageOptions represents page selection options stored in JSONB
type PageOptions struct {
	StartPage  *int    `json:"start_page,omitempty"`  // Starting page number (1-indexed, nil = from first page)
	EndPage    *int    `json:"end_page,omitempty"`    // Ending page number (1-indexed, nil = to last page)
	FilterType *string `json:"filter_type,omitempty"` // Page filter: "all" (default), "odd", "even"
	SkipPages  []int   `json:"skip_pages,omitempty"`  // Array of page numbers (1-indexed) to skip
	ColorPages []int   `json:"color_pages,omitempty"` // Array of page numbers (1-indexed) to print in color
}

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
	PageOptions PageOptions `db:"page_options" json:"page_options,omitempty"` // Consolidated page selection options (JSONB)
	BackToBack *bool `db:"back_to_back" json:"back_to_back,omitempty"` // Nullable - true for duplex printing (both sides), false for simplex (one side)
	DeleteAfterPrint *bool `db:"delete_after_print" json:"delete_after_print,omitempty"` // Nullable - if true, file will be hidden from partner listings after printing is completed
	Status    *string   `db:"status" json:"status,omitempty"`            // Nullable - pending, processing, completed, failed, cancelled
	TotalCost *string   `db:"total_cost" json:"total_cost,omitempty"`    // Nullable (calculated later) - PostgreSQL numeric type
	CreatedAt *time.Time `db:"created_at" json:"created_at,omitempty"`   // Nullable
	UpdatedAt *time.Time `db:"updated_at" json:"updated_at,omitempty"`  // Nullable
}

// ScanPageOptions scans JSONB into PageOptions struct
func ScanPageOptions(src interface{}) (PageOptions, error) {
	var opts PageOptions
	if src == nil {
		return opts, nil
	}
	
	var jsonBytes []byte
	switch v := src.(type) {
	case []byte:
		jsonBytes = v
	case string:
		jsonBytes = []byte(v)
	case map[string]interface{}:
		// Already parsed JSONB object from PostgreSQL
		if len(v) == 0 {
			return opts, nil
		}
		// Convert arrays from []interface{} with float64 to []int
		if skipPagesRaw, ok := v["skip_pages"]; ok && skipPagesRaw != nil {
			if skipPagesArr, ok := skipPagesRaw.([]interface{}); ok {
				skipPages := make([]int, 0, len(skipPagesArr))
				for _, item := range skipPagesArr {
					switch val := item.(type) {
					case float64:
						skipPages = append(skipPages, int(val))
					case int:
						skipPages = append(skipPages, val)
					case int64:
						skipPages = append(skipPages, int(val))
					}
				}
				v["skip_pages"] = skipPages
			}
		}
		if colorPagesRaw, ok := v["color_pages"]; ok && colorPagesRaw != nil {
			if colorPagesArr, ok := colorPagesRaw.([]interface{}); ok {
				colorPages := make([]int, 0, len(colorPagesArr))
				for _, item := range colorPagesArr {
					switch val := item.(type) {
					case float64:
						colorPages = append(colorPages, int(val))
					case int:
						colorPages = append(colorPages, val)
					case int64:
						colorPages = append(colorPages, int(val))
					}
				}
				v["color_pages"] = colorPages
			}
		}
		// Convert map to JSON bytes
		var err error
		jsonBytes, err = json.Marshal(v)
		if err != nil {
			return opts, err
		}
	default:
		// Try to marshal to JSON first
		var err error
		jsonBytes, err = json.Marshal(src)
		if err != nil {
			return opts, err
		}
	}
	
	if len(jsonBytes) == 0 || string(jsonBytes) == "null" || string(jsonBytes) == "{}" {
		return opts, nil
	}
	
	err := json.Unmarshal(jsonBytes, &opts)
	if err != nil {
		return opts, err
	}
	
	// Set default filter_type if not provided
	if opts.FilterType == nil || *opts.FilterType == "" {
		defaultFilter := "all"
		opts.FilterType = &defaultFilter
	}
	
	return opts, nil
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

