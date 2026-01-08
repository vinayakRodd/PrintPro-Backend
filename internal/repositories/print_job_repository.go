package repositories

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"print-pro-backend/internal/models/printjob"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PrintJobRepository handles database operations for print jobs
type PrintJobRepository struct {
	db *pgxpool.Pool
}

// NewPrintJobRepository creates a new print job repository
func NewPrintJobRepository(db *pgxpool.Pool) *PrintJobRepository {
	return &PrintJobRepository{db: db}
}

// Create creates a new print job in the database
func (r *PrintJobRepository) Create(ctx context.Context, accountID *int64, partnerID int64, filename, fileURL string, pType *string, color *bool, numCopies *int, startPage *int, endPage *int, pageFilterType *string, individualColorPages []int, skipPages []int, backToBack *bool, deleteAfterPrint *bool) (*printjob.PrintJob, error) {
	var job printjob.PrintJob
	now := time.Now()
	status := printjob.StatusPending

	// Set defaults if not provided
	var colorValue bool
	if color != nil {
		colorValue = *color
	}

	var numCopiesValue int
	if numCopies != nil {
		numCopiesValue = *numCopies
	} else {
		numCopiesValue = 1 // Default to 1 copy
	}

	// Build page_options JSONB from individual parameters
	pageOpts := printjob.PageOptions{
		StartPage:  startPage,
		EndPage:    endPage,
		SkipPages:  skipPages,
		ColorPages: individualColorPages,
	}
	
	// Set filter_type with default
	if pageFilterType != nil {
		switch *pageFilterType {
		case "all", "odd", "even":
			pageOpts.FilterType = pageFilterType
		default:
			defaultFilter := "all"
			pageOpts.FilterType = &defaultFilter
		}
	} else {
		defaultFilter := "all"
		pageOpts.FilterType = &defaultFilter
	}
	
	// Marshal page_options to JSONB
	pageOptsJSON, err := json.Marshal(pageOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal page_options: %w", err)
	}
	
	var pageOptsJSONB interface{}
	if len(pageOptsJSON) > 0 && string(pageOptsJSON) != "{}" {
		pageOptsJSONB = string(pageOptsJSON)
	} else {
		pageOptsJSONB = "{}" // Empty object
	}

	// Set default for back_to_back if not provided
	var backToBackValue bool
	if backToBack != nil {
		backToBackValue = *backToBack
	} else {
		backToBackValue = false // Default to simplex (one side)
	}

	// Set default for delete_after_print if not provided
	var deleteAfterPrintValue bool
	if deleteAfterPrint != nil {
		deleteAfterPrintValue = *deleteAfterPrint
	} else {
		deleteAfterPrintValue = false // Default to false (show file after printing)
	}

	var pageOptsJSONBScan interface{}
	err = r.db.QueryRow(ctx,
		`INSERT INTO print_jobs (account_id, partner_id, filename, file_url, p_type, color, num_copies, page_options, back_to_back, delete_after_print, status, created_at, updated_at) 
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8::jsonb, $9, $10, $11, $12, $13) 
		 RETURNING id, account_id, partner_id, printer_id, filename, file_url, p_type, color, num_copies, page_options, back_to_back, delete_after_print, status, total_cost, created_at, updated_at`,
		accountID, partnerID, filename, fileURL, pType, colorValue, numCopiesValue, pageOptsJSONB, backToBackValue, deleteAfterPrintValue, status, now, now,
	).Scan(&job.ID, &job.AccountID, &job.PartnerID, &job.PrinterID, &job.Filename, &job.FileURL, &job.PType, &job.Color, &job.NumCopies, &pageOptsJSONBScan, &job.BackToBack, &job.DeleteAfterPrint, &job.Status, &job.TotalCost, &job.CreatedAt, &job.UpdatedAt)
	
	if err != nil {
		return nil, fmt.Errorf("failed to create print job: %w", err)
	}
	
	// Parse page_options from database
	job.PageOptions, err = printjob.ScanPageOptions(pageOptsJSONBScan)
	if err != nil {
		return nil, fmt.Errorf("failed to parse page_options: %w", err)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create print job: %w", err)
	}

	return &job, nil
}

// scanJSONBArray scans a JSONB array column into a []int slice
func scanJSONBArray(src interface{}) ([]int, error) {
	if src == nil {
		return nil, nil
	}
	
	// Handle different types that PostgreSQL might return
	var jsonBytes []byte
	switch v := src.(type) {
	case []byte:
		jsonBytes = v
	case string:
		jsonBytes = []byte(v)
	case []interface{}:
		// PostgreSQL JSONB arrays are sometimes returned as []interface{}
		// Convert []interface{} to []int
		result := make([]int, 0, len(v))
		for _, item := range v {
			switch val := item.(type) {
			case float64:
				// JSON numbers are unmarshaled as float64
				result = append(result, int(val))
			case int:
				result = append(result, val)
			case int64:
				result = append(result, int(val))
			default:
				return nil, fmt.Errorf("cannot convert %T to int in JSONB array", item)
			}
		}
		return result, nil
	default:
		// Try to marshal and unmarshal as fallback
		var err error
		jsonBytes, err = json.Marshal(src)
		if err != nil {
			return nil, fmt.Errorf("cannot scan %T into []int: %w", src, err)
		}
	}
	
	if len(jsonBytes) == 0 {
		return nil, nil
	}
	
	var result []int
	if err := json.Unmarshal(jsonBytes, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSONB array: %w", err)
	}
	
	return result, nil
}

// GetByFilename retrieves a print job by filename
func (r *PrintJobRepository) GetByFilename(ctx context.Context, filename string) (*printjob.PrintJob, error) {
	var job printjob.PrintJob
	var pageOptsJSON interface{}
	
	err := r.db.QueryRow(ctx,
		`SELECT id, account_id, partner_id, printer_id, filename, file_url, p_type, color, num_copies, page_options, back_to_back, delete_after_print, status, total_cost, created_at, updated_at
		 FROM print_jobs 
		 WHERE filename = $1
		 ORDER BY created_at DESC
		 LIMIT 1`,
		filename,
	).Scan(&job.ID, &job.AccountID, &job.PartnerID, &job.PrinterID, &job.Filename, &job.FileURL, &job.PType, &job.Color, &job.NumCopies, &pageOptsJSON, &job.BackToBack, &job.DeleteAfterPrint, &job.Status, &job.TotalCost, &job.CreatedAt, &job.UpdatedAt)

	if err != nil {
		return nil, fmt.Errorf("failed to get print job by filename: %w", err)
	}
	
	// Parse page_options JSONB
	job.PageOptions, err = printjob.ScanPageOptions(pageOptsJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to parse page_options: %w", err)
	}
	
	return &job, nil
}

// GetFilenamesByPartnerID retrieves all filenames for a specific partner
// SECURITY: Only returns files where partner_id matches exactly (not NULL, not 0)
func (r *PrintJobRepository) GetFilenamesByPartnerID(ctx context.Context, partnerID int64) ([]string, error) {
	rows, err := r.db.Query(ctx,
		`SELECT DISTINCT filename, created_at
		 FROM print_jobs 
		 WHERE partner_id = $1 AND partner_id IS NOT NULL
		 ORDER BY created_at DESC`,
		partnerID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get filenames by partner_id: %w", err)
	}
	defer rows.Close()

	var filenames []string
	for rows.Next() {
		var filename string
		var createdAt time.Time
		if err := rows.Scan(&filename, &createdAt); err != nil {
			return nil, fmt.Errorf("failed to scan filename: %w", err)
		}
		filenames = append(filenames, filename)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating filenames: %w", err)
	}

	return filenames, nil
}

// scanPrintJobRow scans a single row from a query result into a PrintJob struct
func (r *PrintJobRepository) scanPrintJobRow(rows interface {
	Scan(dest ...interface{}) error
}) (*printjob.PrintJob, error) {
	var job printjob.PrintJob
	var pageOptsJSON interface{}
	
	err := rows.Scan(
		&job.ID, &job.AccountID, &job.PartnerID, &job.PrinterID, &job.Filename, &job.FileURL,
		&job.PType, &job.Color, &job.NumCopies, &pageOptsJSON, &job.BackToBack, &job.DeleteAfterPrint, &job.Status, &job.TotalCost, &job.CreatedAt, &job.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	
	// Parse page_options JSONB
	job.PageOptions, err = printjob.ScanPageOptions(pageOptsJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to parse page_options: %w", err)
	}

	return &job, nil
}

// GetByPartnerID retrieves all print jobs for a specific partner
// SECURITY: Only returns files where partner_id matches exactly (not NULL, not 0)
// Uses strict equality check to prevent any cross-shop data leakage
// Filters out completed jobs where delete_after_print = true
func (r *PrintJobRepository) GetByPartnerID(ctx context.Context, partnerID int64) ([]printjob.PrintJob, error) {
	// SECURITY: Use strict WHERE clause with explicit type checking
	// Filter out completed jobs where delete_after_print = true (files should be hidden after printing)
	// IMPORTANT: Only hide files when BOTH conditions are true: delete_after_print = true AND status = 'completed'
	// Files with NULL status or other statuses should still be visible even if delete_after_print = true
	rows, err := r.db.Query(ctx,
		`SELECT id, account_id, partner_id, printer_id, filename, file_url, p_type, color, num_copies, page_options, back_to_back, delete_after_print, status, total_cost, created_at, updated_at
		 FROM print_jobs 
		 WHERE partner_id = $1::bigint AND partner_id IS NOT NULL
		   AND NOT (delete_after_print = true AND COALESCE(status, '') = 'completed')
		 ORDER BY created_at DESC`,
		partnerID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get print jobs by partner_id: %w", err)
	}
	defer rows.Close()

	var jobs []printjob.PrintJob
	for rows.Next() {
		job, err := r.scanPrintJobRow(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan print job: %w", err)
		}
		
		// SECURITY: Double-check partner_id matches (defensive programming)
		// This should never happen if the SQL query is correct, but we verify anyway
		if job.PartnerID != partnerID {
			// Log error - this indicates a serious database integrity issue
			// Skip this job - it doesn't belong to this partner
			continue
		}
		
		jobs = append(jobs, *job)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating print jobs: %w", err)
	}

	return jobs, nil
}

// UpdateStatus updates the status of a print job by filename
func (r *PrintJobRepository) UpdateStatus(ctx context.Context, filename, status string) error {
	now := time.Now()
	_, err := r.db.Exec(ctx,
		`UPDATE print_jobs 
		 SET status = $1, updated_at = $2 
		 WHERE filename = $3`,
		status, now, filename,
	)
	if err != nil {
		return fmt.Errorf("failed to update print job status: %w", err)
	}
	return nil
}

// UpdatePrintOptions updates print job options (color, num_copies, start_page, end_page)
// SECURITY: Only updates if the file belongs to the specified partner_id
// For customers: accountID should be provided to verify ownership
// Only updates fields that are provided (non-nil)
func (r *PrintJobRepository) UpdatePrintOptions(ctx context.Context, filename string, partnerID int64, accountID *int64, color *bool, numCopies *int, startPage *int, endPage *int, pageFilterType *string, individualColorPages *[]int, skipPages *[]int, backToBack *bool, deleteAfterPrint *bool) error {
	// Build dynamic UPDATE query - only update fields that are provided (non-nil)
	updates := []string{}
	args := []interface{}{}
	argIndex := 1
	
	if color != nil {
		updates = append(updates, fmt.Sprintf("color = $%d", argIndex))
		args = append(args, *color)
		argIndex++
	}
	
	if numCopies != nil {
		updates = append(updates, fmt.Sprintf("num_copies = $%d::integer", argIndex))
		args = append(args, *numCopies)
		argIndex++
	}
	
	// Handle page_options update - merge with existing page_options
	// We need to get existing page_options first, then merge with new values
	pageOptsUpdated := false
	var existingPageOpts printjob.PageOptions
	
	// Check if any page-related fields are being updated
	if startPage != nil || endPage != nil || pageFilterType != nil || individualColorPages != nil || skipPages != nil {
		// Get existing print job to merge page_options
		existingJob, err := r.GetByFilename(ctx, filename)
		if err == nil {
			existingPageOpts = existingJob.PageOptions
		}
		
		// Build updated page_options by merging existing with new values
		updatedPageOpts := existingPageOpts
		
		if startPage != nil {
			updatedPageOpts.StartPage = startPage
			pageOptsUpdated = true
		}
		if endPage != nil {
			updatedPageOpts.EndPage = endPage
			pageOptsUpdated = true
		}
		if pageFilterType != nil {
			updatedPageOpts.FilterType = pageFilterType
			pageOptsUpdated = true
		}
		if individualColorPages != nil {
			if len(*individualColorPages) == 0 {
				updatedPageOpts.ColorPages = nil // Clear
			} else {
				updatedPageOpts.ColorPages = *individualColorPages
			}
			pageOptsUpdated = true
		}
		if skipPages != nil {
			if len(*skipPages) == 0 {
				updatedPageOpts.SkipPages = nil // Clear
			} else {
				updatedPageOpts.SkipPages = *skipPages
			}
			pageOptsUpdated = true
		}
		
		if pageOptsUpdated {
			// Marshal updated page_options to JSONB
			pageOptsJSON, err := json.Marshal(updatedPageOpts)
			if err != nil {
				return fmt.Errorf("failed to marshal page_options: %w", err)
			}
			updates = append(updates, fmt.Sprintf("page_options = $%d::jsonb", argIndex))
			args = append(args, string(pageOptsJSON))
			argIndex++
		}
	}
	
	// Handle back_to_back update
	if backToBack != nil {
		updates = append(updates, fmt.Sprintf("back_to_back = $%d", argIndex))
		args = append(args, *backToBack)
		argIndex++
	}
	// If backToBack is nil pointer, don't add to updates (don't change existing value)
	
	// Handle delete_after_print update
	if deleteAfterPrint != nil {
		updates = append(updates, fmt.Sprintf("delete_after_print = $%d", argIndex))
		args = append(args, *deleteAfterPrint)
		argIndex++
	}
	// If deleteAfterPrint is nil pointer, don't add to updates (don't change existing value)
	
	// If no fields to update, return early
	if len(updates) == 0 {
		return fmt.Errorf("no fields to update")
	}
	
	// Add updated_at (always update timestamp)
	// Use CURRENT_TIMESTAMP to avoid type casting issues with dynamic queries
	updates = append(updates, "updated_at = CURRENT_TIMESTAMP")
	
	// Build WHERE clause with security checks
	// Parameter indices continue from where SET clause left off
	whereParamIndex := argIndex
	args = append(args, filename, partnerID)
	whereClause := fmt.Sprintf("filename = $%d AND partner_id = $%d::bigint", whereParamIndex, whereParamIndex+1)
	
	// If accountID is provided (for customer verification), add account_id check
	if accountID != nil {
		args = append(args, *accountID)
		whereClause += fmt.Sprintf(" AND account_id = $%d::bigint", whereParamIndex+2)
	}
	
	// Build the SET clause
	setClause := strings.Join(updates, ", ")
	
	query := fmt.Sprintf(
		`UPDATE print_jobs 
		 SET %s 
		 WHERE %s`,
		setClause,
		whereClause,
	)
	
	result, err := r.db.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to update print job options: %w", err)
	}
	
	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		if accountID != nil {
			return fmt.Errorf("print job not found or does not belong to this customer")
		}
		return fmt.Errorf("print job not found or does not belong to this partner")
	}
	
	return nil
}

// GetByAccountID retrieves all print jobs for a specific customer (account_id)
func (r *PrintJobRepository) GetByAccountID(ctx context.Context, accountID int64) ([]printjob.PrintJob, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, account_id, partner_id, printer_id, filename, file_url, p_type, color, num_copies, page_options, back_to_back, delete_after_print, status, total_cost, created_at, updated_at
		 FROM print_jobs 
		 WHERE account_id = $1
		 ORDER BY created_at DESC`,
		accountID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get print jobs by account_id: %w", err)
	}
	defer rows.Close()

	var jobs []printjob.PrintJob
	for rows.Next() {
		job, err := r.scanPrintJobRow(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan print job: %w", err)
		}
		jobs = append(jobs, *job)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating print jobs: %w", err)
	}

	return jobs, nil
}

// GetByAccountIDAndPartnerID retrieves all print jobs for a specific customer (account_id) AND shop (partner_id)
// SECURITY: This ensures customers only see files they uploaded to a specific shop
// Excludes completed files from the results
func (r *PrintJobRepository) GetByAccountIDAndPartnerID(ctx context.Context, accountID int64, partnerID int64) ([]printjob.PrintJob, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, account_id, partner_id, printer_id, filename, file_url, p_type, color, num_copies, page_options, back_to_back, delete_after_print, status, total_cost, created_at, updated_at
		 FROM print_jobs 
		 WHERE account_id = $1 AND partner_id = $2 AND partner_id IS NOT NULL AND (status IS NULL OR status != 'completed')
		 ORDER BY created_at DESC`,
		accountID, partnerID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get print jobs by account_id and partner_id: %w", err)
	}
	defer rows.Close()

	var jobs []printjob.PrintJob
	for rows.Next() {
		job, err := r.scanPrintJobRow(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan print job: %w", err)
		}
		// Security verification: double-check partner_id matches
		if job.PartnerID != partnerID {
			return nil, fmt.Errorf("security error: print job %d has partner_id %d but expected %d", job.ID, job.PartnerID, partnerID)
		}
		jobs = append(jobs, *job)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating print jobs: %w", err)
	}

	return jobs, nil
}

// DeleteByFilenameAndAccountID deletes a print job by filename, but only if it belongs to the specified account_id
// This ensures customers can only delete their own files
func (r *PrintJobRepository) DeleteByFilenameAndAccountID(ctx context.Context, filename string, accountID int64) error {
	result, err := r.db.Exec(ctx,
		`DELETE FROM print_jobs 
		 WHERE filename = $1 AND account_id = $2`,
		filename, accountID,
	)
	if err != nil {
		return fmt.Errorf("failed to delete print job: %w", err)
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("print job not found or does not belong to this account")
	}

	return nil
}

