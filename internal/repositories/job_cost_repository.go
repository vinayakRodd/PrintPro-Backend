package repositories

import (
	"context"
	"fmt"
	"log"
	"time"

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
// print_job_id is the primary key, customer_email is a foreign key reference
func (r *JobCostRepository) CreateOrUpdate(ctx context.Context, accountEmail string, printJobID int64, cost *jobcost.JobCost) error {
	query := `
		INSERT INTO job_cost (
			print_job_id, customer_email, total_pages, pages_to_print, color_pages, black_white_pages,
			num_copies, total_cost, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		ON CONFLICT (print_job_id)
		DO UPDATE SET
			customer_email = EXCLUDED.customer_email,
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
		accountEmail,
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

	log.Printf("Job cost created/updated for customer_email: %s, print_job_id: %d, Total cost: %.2f", accountEmail, printJobID, cost.TotalCost)
	return nil
}

// GetByAccountEmail retrieves job cost by customer email (foreign key lookup)
func (r *JobCostRepository) GetByAccountEmail(ctx context.Context, accountEmail string) (*jobcost.JobCost, error) {
	var cost jobcost.JobCost

	err := r.db.QueryRow(ctx,
		`SELECT print_job_id, customer_email, total_pages, pages_to_print, color_pages, black_white_pages,
		        num_copies, total_cost, created_at, updated_at
		 FROM job_cost
		 WHERE customer_email = $1`,
		accountEmail,
	).Scan(
		&cost.PrintJobID, &cost.AccountEmail, &cost.TotalPages, &cost.PagesToPrint,
		&cost.ColorPages, &cost.BlackWhitePages, &cost.NumCopies,
		&cost.TotalCost, &cost.CreatedAt, &cost.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get job cost: %w", err)
	}

	return &cost, nil
}

// GetByPrintJobID retrieves job cost by print job ID (primary key lookup)
func (r *JobCostRepository) GetByPrintJobID(ctx context.Context, printJobID int64) (*jobcost.JobCost, error) {
	var cost jobcost.JobCost

	err := r.db.QueryRow(ctx,
		`SELECT print_job_id, customer_email, total_pages, pages_to_print, color_pages, black_white_pages,
		        num_copies, total_cost, created_at, updated_at
		 FROM job_cost
		 WHERE print_job_id = $1`,
		printJobID,
	).Scan(
		&cost.PrintJobID, &cost.AccountEmail, &cost.TotalPages, &cost.PagesToPrint,
		&cost.ColorPages, &cost.BlackWhitePages, &cost.NumCopies,
		&cost.TotalCost, &cost.CreatedAt, &cost.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get job cost: %w", err)
	}

	return &cost, nil
}

// GetAll retrieves all job costs from the job_cost table
// Optional filter by customer_email - if provided, returns only costs for that customer
func (r *JobCostRepository) GetAll(ctx context.Context, customerEmailFilter *string) ([]jobcost.JobCost, error) {
	var query string
	var args []interface{}
	
	if customerEmailFilter != nil && *customerEmailFilter != "" {
		// Filter by customer email
		query = `
			SELECT print_job_id, customer_email, total_pages, pages_to_print, color_pages, black_white_pages,
			       num_copies, total_cost, created_at, updated_at
			FROM job_cost
			WHERE customer_email = $1
			ORDER BY created_at DESC
		`
		args = []interface{}{*customerEmailFilter}
	} else {
		// Get all job costs
		query = `
			SELECT print_job_id, customer_email, total_pages, pages_to_print, color_pages, black_white_pages,
			       num_copies, total_cost, created_at, updated_at
			FROM job_cost
			ORDER BY created_at DESC
		`
		args = []interface{}{}
	}
	
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get job costs: %w", err)
	}
	defer rows.Close()
	
	var costs []jobcost.JobCost
	for rows.Next() {
		var cost jobcost.JobCost
		err := rows.Scan(
			&cost.PrintJobID, &cost.AccountEmail, &cost.TotalPages, &cost.PagesToPrint,
			&cost.ColorPages, &cost.BlackWhitePages, &cost.NumCopies,
			&cost.TotalCost, &cost.CreatedAt, &cost.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan job cost: %w", err)
		}
		costs = append(costs, cost)
	}
	
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating job costs: %w", err)
	}
	
	return costs, nil
}

// GetByPartnerEmail retrieves all job costs for jobs belonging to a partner's shop
// Joins with print_jobs table to filter by partner_email
func (r *JobCostRepository) GetByPartnerEmail(ctx context.Context, partnerEmail string) ([]jobcost.JobCost, error) {
	query := `
		SELECT jc.print_job_id, jc.customer_email, jc.total_pages, jc.pages_to_print, jc.color_pages, jc.black_white_pages,
		       jc.num_copies, jc.total_cost, jc.created_at, jc.updated_at
		FROM job_cost jc
		INNER JOIN print_jobs pj ON jc.print_job_id = pj.id
		WHERE pj.partner_email = $1
		ORDER BY jc.created_at DESC
	`
	
	rows, err := r.db.Query(ctx, query, partnerEmail)
	if err != nil {
		return nil, fmt.Errorf("failed to get job costs by partner email: %w", err)
	}
	defer rows.Close()
	
	var costs []jobcost.JobCost
	for rows.Next() {
		var cost jobcost.JobCost
		err := rows.Scan(
			&cost.PrintJobID, &cost.AccountEmail, &cost.TotalPages, &cost.PagesToPrint,
			&cost.ColorPages, &cost.BlackWhitePages, &cost.NumCopies,
			&cost.TotalCost, &cost.CreatedAt, &cost.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan job cost: %w", err)
		}
		costs = append(costs, cost)
	}
	
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating job costs: %w", err)
	}
	
	return costs, nil
}

// GetByMonth retrieves all job costs for a specific month and year
// Filters by created_at timestamp to get invoices for that month
func (r *JobCostRepository) GetByMonth(ctx context.Context, year int, month int) ([]jobcost.JobCost, error) {
	// Calculate the first day of the target month
	startOfMonth := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	
	// Calculate the first day of the NEXT month (automatically handles Dec -> Jan)
	startOfNextMonth := startOfMonth.AddDate(0, 1, 0)
	
	query := `
		SELECT print_job_id, customer_email, total_pages, pages_to_print, color_pages, black_white_pages,
		       num_copies, total_cost, created_at, updated_at
		FROM job_cost
		WHERE created_at >= $1 AND created_at < $2
		ORDER BY created_at DESC
	`
	
	rows, err := r.db.Query(ctx, query, startOfMonth, startOfNextMonth)
	if err != nil {
		return nil, fmt.Errorf("failed to get job costs by month: %w", err)
	}
	defer rows.Close()
	
	var costs []jobcost.JobCost
	for rows.Next() {
		var cost jobcost.JobCost
		err := rows.Scan(
			&cost.PrintJobID, &cost.AccountEmail, &cost.TotalPages, &cost.PagesToPrint,
			&cost.ColorPages, &cost.BlackWhitePages, &cost.NumCopies,
			&cost.TotalCost, &cost.CreatedAt, &cost.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan job cost: %w", err)
		}
		costs = append(costs, cost)
	}
	
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating job costs: %w", err)
	}
	
	return costs, nil
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
