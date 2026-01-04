package repositories

import (
	"context"
	"fmt"
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
func (r *PrintJobRepository) Create(ctx context.Context, accountID *int64, partnerID int64, filename, fileURL string, pType *string, color *bool, numCopies *int) (*printjob.PrintJob, error) {
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

	err := r.db.QueryRow(ctx,
		`INSERT INTO print_jobs (account_id, partner_id, filename, file_url, p_type, color, num_copies, status, created_at, updated_at) 
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10) 
		 RETURNING id, account_id, partner_id, printer_id, filename, file_url, p_type, color, num_copies, status, total_cost, created_at, updated_at`,
		accountID, partnerID, filename, fileURL, pType, colorValue, numCopiesValue, status, now, now,
	).Scan(&job.ID, &job.AccountID, &job.PartnerID, &job.PrinterID, &job.Filename, &job.FileURL, &job.PType, &job.Color, &job.NumCopies, &job.Status, &job.TotalCost, &job.CreatedAt, &job.UpdatedAt)

	if err != nil {
		return nil, fmt.Errorf("failed to create print job: %w", err)
	}

	return &job, nil
}

// GetByFilename retrieves a print job by filename
func (r *PrintJobRepository) GetByFilename(ctx context.Context, filename string) (*printjob.PrintJob, error) {
	var job printjob.PrintJob
	err := r.db.QueryRow(ctx,
		`SELECT id, account_id, partner_id, printer_id, filename, file_url, p_type, color, num_copies, status, total_cost, created_at, updated_at
		 FROM print_jobs 
		 WHERE filename = $1
		 ORDER BY created_at DESC
		 LIMIT 1`,
		filename,
	).Scan(&job.ID, &job.AccountID, &job.PartnerID, &job.PrinterID, &job.Filename, &job.FileURL, &job.PType, &job.Color, &job.NumCopies, &job.Status, &job.TotalCost, &job.CreatedAt, &job.UpdatedAt)

	if err != nil {
		return nil, fmt.Errorf("failed to get print job by filename: %w", err)
	}

	return &job, nil
}

// GetFilenamesByPartnerID retrieves all filenames for a specific partner
func (r *PrintJobRepository) GetFilenamesByPartnerID(ctx context.Context, partnerID int64) ([]string, error) {
	rows, err := r.db.Query(ctx,
		`SELECT DISTINCT filename, created_at
		 FROM print_jobs 
		 WHERE partner_id = $1
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

