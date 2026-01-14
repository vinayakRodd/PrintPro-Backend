package repositories

import (
	"context"
	"fmt"
	"print-pro-backend/internal/models/printer"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PrinterRepository handles database operations for printers
type PrinterRepository struct {
	db *pgxpool.Pool
}

// NewPrinterRepository creates a new printer repository
func NewPrinterRepository(db *pgxpool.Pool) *PrinterRepository {
	return &PrinterRepository{db: db}
}

// UpsertPrinter inserts a new printer or updates an existing one based on serial_number
// Uses ON CONFLICT to update last_seen and status if the printer already exists
func (r *PrinterRepository) UpsertPrinter(ctx context.Context, partnerEmail *string, printerName, serialNumber, status string) (*printer.Printer, error) {
	var p printer.Printer
	now := time.Now()
	
	err := r.db.QueryRow(ctx,
		`INSERT INTO printers (partner_email, printer_name, serial_number, status, last_seen, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 ON CONFLICT (serial_number) 
		 DO UPDATE SET 
			partner_email = EXCLUDED.partner_email,
			status = EXCLUDED.status,
			last_seen = EXCLUDED.last_seen,
			updated_at = EXCLUDED.updated_at
		 RETURNING id, partner_email, printer_name, serial_number, status, last_seen, created_at, updated_at`,
		partnerEmail, printerName, serialNumber, status, now, now,
	).Scan(&p.ID, &p.PartnerEmail, &p.PrinterName, &p.SerialNumber, &p.Status, &p.LastSeen, &p.CreatedAt, &p.UpdatedAt)

	if err != nil {
		return nil, fmt.Errorf("failed to upsert printer: %w", err)
	}

	return &p, nil
}

// GetByAccountEmail retrieves all printers for a specific partner account
func (r *PrinterRepository) GetByAccountEmail(ctx context.Context, accountEmail string) ([]printer.Printer, error) {
	rows, err := r.db.Query(ctx,
		"SELECT id, partner_email, printer_name, serial_number, status, last_seen, created_at, updated_at FROM printers WHERE partner_email = $1 ORDER BY last_seen DESC",
		accountEmail,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get printers: %w", err)
	}
	defer rows.Close()

	var printers []printer.Printer
	for rows.Next() {
		var p printer.Printer
		if err := rows.Scan(&p.ID, &p.PartnerEmail, &p.PrinterName, &p.SerialNumber, &p.Status, &p.LastSeen, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan printer: %w", err)
		}
		printers = append(printers, p)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating printers: %w", err)
	}

	return printers, nil
}

// GetByPartnerID is deprecated - use GetByAccountEmail instead
func (r *PrinterRepository) GetByPartnerID(ctx context.Context, partnerID int64) ([]printer.Printer, error) {
	return nil, fmt.Errorf("GetByPartnerID is deprecated, use GetByAccountEmail instead")
}

