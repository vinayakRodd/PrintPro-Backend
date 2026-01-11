package repositories

import (
	"context"
	"fmt"
	"log"

	"print-pro-backend/internal/models/jobcost"
	"github.com/jackc/pgx/v5/pgxpool"
)

// JobCostRepository handles database operations for job costs
type JobCostRepository struct {
	db *pgxpool.Pool
}

// NewJobCostRepository creates a new job cost repository
func NewJobCostRepository(db *pgxpool.Pool) *JobCostRepository {
	return &JobCostRepository{
		db: db,
	}
}

// CreateOrUpdate creates or updates a job cost record
func (r *JobCostRepository) CreateOrUpdate(ctx context.Context, printJobID int64, cost *jobcost.JobCost) error {
	query := `
		INSERT INTO job_cost (
			print_job_id, total_pages, pages_to_print, color_pages, black_white_pages,
			num_copies, total_cost, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		ON CONFLICT (print_job_id)
		DO UPDATE SET
			total_pages = EXCLUDED.total_pages,
			pages_to_print = EXCLUDED.pages_to_print,
			color_pages = EXCLUDED.color_pages,
			black_white_pages = EXCLUDED.black_white_pages,
			num_copies = EXCLUDED.num_copies,
			total_cost = EXCLUDED.total_cost,
			updated_at = CURRENT_TIMESTAMP
	`

	_, err := r.db.Exec(ctx, query,
		printJobID,
		cost.TotalPages,
		cost.PagesToPrint,
		cost.ColorPages,
		cost.BlackWhitePages,
		cost.NumCopies,
		cost.TotalCost,
	)

	if err != nil {
		return fmt.Errorf("failed to create or update job cost: %w", err)
	}

	log.Printf("Job cost created/updated for print_job_id: %d, Total cost: %.2f", printJobID, cost.TotalCost)
	return nil
}

// GetByPrintJobID retrieves job cost by print job ID
func (r *JobCostRepository) GetByPrintJobID(ctx context.Context, printJobID int64) (*jobcost.JobCost, error) {
	var cost jobcost.JobCost

	err := r.db.QueryRow(ctx,
		`SELECT id, print_job_id, total_pages, pages_to_print, color_pages, black_white_pages,
		        num_copies, total_cost, created_at, updated_at
		 FROM job_cost
		 WHERE print_job_id = $1`,
		printJobID,
	).Scan(
		&cost.ID, &cost.PrintJobID, &cost.TotalPages, &cost.PagesToPrint,
		&cost.ColorPages, &cost.BlackWhitePages, &cost.NumCopies,
		&cost.TotalCost, &cost.CreatedAt, &cost.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get job cost: %w", err)
	}

	return &cost, nil
}

// UpdateTotalCostInPrintJob updates the total_cost column in print_jobs table
func (r *JobCostRepository) UpdateTotalCostInPrintJob(ctx context.Context, printJobID int64, totalCost float64) error {
	query := `UPDATE print_jobs SET total_cost = $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2`
	
	_, err := r.db.Exec(ctx, query, fmt.Sprintf("%.2f", totalCost), printJobID)
	if err != nil {
		return fmt.Errorf("failed to update total_cost in print_jobs: %w", err)
	}

	return nil
}
