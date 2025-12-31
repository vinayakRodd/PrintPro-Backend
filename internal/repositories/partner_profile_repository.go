package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"print-pro-backend/internal/models/partner_profile"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PartnerProfileRepository handles database operations for partner profiles
type PartnerProfileRepository struct {
	db *pgxpool.Pool
}

// NewPartnerProfileRepository creates a new partner profile repository
func NewPartnerProfileRepository(db *pgxpool.Pool) *PartnerProfileRepository {
	return &PartnerProfileRepository{db: db}
}

// Create creates a new partner profile
func (r *PartnerProfileRepository) Create(ctx context.Context, accountID int, shopName, printerID string) (*partner_profile.PartnerProfile, error) {
	var p partner_profile.PartnerProfile
	err := r.db.QueryRow(ctx,
		`INSERT INTO partner_profiles (account_id, shop_name, printer_id) 
		 VALUES ($1, $2, $3) 
		 RETURNING id, account_id, shop_name, printer_id`,
		accountID, shopName, printerID,
	).Scan(&p.ID, &p.AccountID, &p.ShopName, &p.PrinterID)

	if err != nil {
		return nil, fmt.Errorf("failed to create partner profile: %w", err)
	}

	return &p, nil
}

// GetByAccountID retrieves a partner profile by account ID
func (r *PartnerProfileRepository) GetByAccountID(ctx context.Context, accountID int) (*partner_profile.PartnerProfile, error) {
	var p partner_profile.PartnerProfile
	err := r.db.QueryRow(ctx,
		"SELECT id, account_id, shop_name, printer_id FROM partner_profiles WHERE account_id = $1",
		accountID,
	).Scan(&p.ID, &p.AccountID, &p.ShopName, &p.PrinterID)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("partner profile not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get partner profile: %w", err)
	}

	return &p, nil
}

