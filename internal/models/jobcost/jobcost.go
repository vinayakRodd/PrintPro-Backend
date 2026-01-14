package jobcost

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

// JobCost represents a cost breakdown for a print job
type JobCost struct {
	AccountEmail    string     `db:"customer_email" json:"customer_email"` // Primary key, references accounts(email)
	PrintJobID      int64      `db:"print_job_id" json:"print_job_id"`   // References print_jobs(id)
	TotalPages      int        `db:"total_pages" json:"total_pages"`           // Total pages in document
	PagesToPrint    int        `db:"pages_to_print" json:"pages_to_print"`     // Pages that will be printed (after filters)
	ColorPages      int        `db:"color_pages" json:"color_pages"`           // Number of color pages (individual_color_pages_count)
	BlackWhitePages int        `db:"black_white_pages" json:"black_white_pages"` // Number of B&W pages (total_pages - skip_pages - individual_color_pages_count)
	NumCopies       int        `db:"num_copies" json:"num_copies"`             // Number of copies
	TotalCost       float64    `db:"total_cost" json:"total_cost"`             // Total calculated cost
	CreatedAt       *time.Time `db:"created_at" json:"created_at,omitempty"`
	UpdatedAt       *time.Time `db:"updated_at" json:"updated_at,omitempty"`
}

// CalculateTotalCost calculates the total cost based on page counts and standard pricing
// Formula: total_cost = (bw_cost*(total_pages-individual_color_pages-skip_pages) + color_page_cost*individual_color_pages)*num_copies
func (jc *JobCost) CalculateTotalCost(colorCostPerPage, bwCostPerPage float64) {
	// B&W pages = total_pages - individual_color_pages - skip_pages
	// Color pages = individual_color_pages
	bwPages := jc.BlackWhitePages
	colorPages := jc.ColorPages
	
	// Calculate cost: (bw_cost * bw_pages + color_cost * color_pages) * num_copies
	bwCost := float64(bwPages) * bwCostPerPage
	colorCost := float64(colorPages) * colorCostPerPage
	jc.TotalCost = (bwCost + colorCost) * float64(jc.NumCopies)
}

// Value implements driver.Valuer for database storage
func (jc JobCost) Value() (driver.Value, error) {
	return json.Marshal(jc)
}

// Scan implements sql.Scanner for database retrieval
func (jc *JobCost) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("failed to unmarshal JobCost value: %v", value)
	}
	return json.Unmarshal(bytes, jc)
}
