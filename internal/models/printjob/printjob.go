package printjob

import (
	"encoding/json"
	"fmt"
	"strings"
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

// CropOptions represents crop margins in millimeters stored in JSONB
type CropOptions struct {
	Top    float64 `json:"top"`    // Top crop margin in mm (default: 0)
	Bottom float64 `json:"bottom"` // Bottom crop margin in mm (default: 0)
	Left   float64 `json:"left"`   // Left crop margin in mm (default: 0)
	Right  float64 `json:"right"`   // Right crop margin in mm (default: 0)
}

// ScanCropOptions scans JSONB into CropOptions struct
func ScanCropOptions(src interface{}) (CropOptions, error) {
	var opts CropOptions
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
		// Convert float64 values to float64 (JSON numbers are float64)
		if top, ok := v["top"].(float64); ok {
			opts.Top = top
		}
		if bottom, ok := v["bottom"].(float64); ok {
			opts.Bottom = bottom
		}
		if left, ok := v["left"].(float64); ok {
			opts.Left = left
		}
		if right, ok := v["right"].(float64); ok {
			opts.Right = right
		}
		return opts, nil
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
	
	return opts, nil
}

// PrintJob represents a print job in the database
type PrintJob struct {
	ID        int64     `db:"id" json:"id"`
	AccountID *int64    `db:"account_id" json:"account_id,omitempty"`     // Nullable (customer account ID)
	PartnerID int64     `db:"partner_id" json:"partner_id"`              // Required - partner account ID
	PrinterID *int64    `db:"printer_id" json:"printer_id,omitempty"`    // Nullable (assigned later)
	Filename  string    `db:"filename" json:"filename"`
	FileURL   string    `db:"file_url" json:"file_url"`
	PType     *string   `db:"p_type" json:"p_type,omitempty"`            // Nullable - print type (e.g., "A4", "A3", etc.) - legacy field name
	PrintType *string   `db:"print_type" json:"print_type,omitempty"`    // Nullable - paper size (e.g., "A4", "Letter", "A3") - preferred field name
	Color     *bool     `db:"color" json:"color,omitempty"`              // Nullable - color printing (default: false)
	NumCopies *int      `db:"num_copies" json:"num_copies,omitempty"`    // Nullable - number of copies (default: 1)
	PageOptions PageOptions `db:"page_options" json:"page_options,omitempty"` // Consolidated page selection options (JSONB)
	CropOptions *CropOptions `db:"crop_options" json:"crop_options,omitempty"` // Crop margins in millimeters (JSONB)
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

// Paper size constants
const (
	PaperSizeA4             = "A4"
	PaperSizeA3             = "A3"
	PaperSizeA5             = "A5"
	PaperSizeA6             = "A6"
	PaperSizeB4             = "B4"
	PaperSizeB5             = "B5"
	PaperSizeLetter         = "Letter"
	PaperSizeLegal          = "Legal"
	PaperSizeTabloid        = "Tabloid"
	PaperSizeExecutive      = "Executive"
	PaperSizeA4Landscape    = "A4_Landscape"
	PaperSizeLetterLandscape = "Letter_Landscape"
	PaperSizeDefault        = PaperSizeA4 // Default paper size
)

// ValidPaperSizes is a list of all valid paper size names
var ValidPaperSizes = []string{
	PaperSizeA4,
	PaperSizeA3,
	PaperSizeA5,
	PaperSizeA6,
	PaperSizeB4,
	PaperSizeB5,
	PaperSizeLetter,
	PaperSizeLegal,
	PaperSizeTabloid,
	PaperSizeExecutive,
	PaperSizeA4Landscape,
	PaperSizeLetterLandscape,
}

// PaperDimensions represents width and height in millimeters
type PaperDimensions struct {
	Width  float64 // Width in millimeters
	Height float64 // Height in millimeters
}

// GetPaperDimensions returns the dimensions for a given paper size
func GetPaperDimensions(paperSize string) PaperDimensions {
	// Paper size dimensions in millimeters
	dimensions := map[string]PaperDimensions{
		PaperSizeA4:             {Width: 210, Height: 297},
		PaperSizeA3:             {Width: 297, Height: 420},
		PaperSizeA5:             {Width: 148, Height: 210},
		PaperSizeA6:             {Width: 105, Height: 148},
		PaperSizeB4:             {Width: 250, Height: 353},
		PaperSizeB5:             {Width: 176, Height: 250},
		PaperSizeLetter:         {Width: 216, Height: 279},
		PaperSizeLegal:          {Width: 216, Height: 356},
		PaperSizeTabloid:        {Width: 279, Height: 432},
		PaperSizeExecutive:      {Width: 184, Height: 267},
		PaperSizeA4Landscape:    {Width: 297, Height: 210},
		PaperSizeLetterLandscape: {Width: 279, Height: 216},
	}
	
	if dims, ok := dimensions[paperSize]; ok {
		return dims
	}
	// Default to A4 if not found
	return dimensions[PaperSizeA4]
}

// ValidatePaperSize validates and normalizes a paper size string
// Returns the normalized paper size or default (A4) if invalid
func ValidatePaperSize(printType string) string {
	if printType == "" {
		return PaperSizeDefault
	}
	
	normalized := strings.TrimSpace(printType)
	// Case-insensitive comparison
	for _, validSize := range ValidPaperSizes {
		if strings.EqualFold(normalized, validSize) {
			return validSize
		}
	}
	
	// If invalid, return default
	return PaperSizeDefault
}


// ValidateCropOptions validates crop options against paper size dimensions
// Returns validated crop options with clamped values
func ValidateCropOptions(cropOpts *CropOptions, paperSize string) (*CropOptions, error) {
	if cropOpts == nil {
		// Return default (no cropping)
		return &CropOptions{Top: 0, Bottom: 0, Left: 0, Right: 0}, nil
	}
	
	// Get paper dimensions
	dims := GetPaperDimensions(paperSize)
	widthMM := dims.Width
	heightMM := dims.Height
	
	// Validate and clamp values (must be non-negative)
	top := cropOpts.Top
	if top < 0 {
		top = 0
	}
	bottom := cropOpts.Bottom
	if bottom < 0 {
		bottom = 0
	}
	left := cropOpts.Left
	if left < 0 {
		left = 0
	}
	right := cropOpts.Right
	if right < 0 {
		right = 0
	}
	
	// Ensure crop values don't exceed paper dimensions (max 50% of dimension)
	maxTop := heightMM * 0.5
	maxBottom := heightMM * 0.5
	maxLeft := widthMM * 0.5
	maxRight := widthMM * 0.5
	
	if top > maxTop {
		top = maxTop
	}
	if bottom > maxBottom {
		bottom = maxBottom
	}
	if left > maxLeft {
		left = maxLeft
	}
	if right > maxRight {
		right = maxRight
	}
	
	// Ensure printable area is at least 10% of original size
	printableWidth := widthMM - left - right
	printableHeight := heightMM - top - bottom
	
	if printableWidth < widthMM*0.1 {
		return nil, fmt.Errorf("crop margins too large: printable width would be less than 10%% of paper width")
	}
	if printableHeight < heightMM*0.1 {
		return nil, fmt.Errorf("crop margins too large: printable height would be less than 10%% of paper height")
	}
	
	// Round to 1 decimal place
	return &CropOptions{
		Top:    roundTo1Decimal(top),
		Bottom: roundTo1Decimal(bottom),
		Left:   roundTo1Decimal(left),
		Right:  roundTo1Decimal(right),
	}, nil
}

// roundTo1Decimal rounds a float64 to 1 decimal place
func roundTo1Decimal(val float64) float64 {
	return float64(int(val*10+0.5)) / 10.0
}