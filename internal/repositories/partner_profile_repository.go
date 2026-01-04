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
func (r *PartnerProfileRepository) Create(ctx context.Context, accountID int64, shopName, printerID string) (*partner_profile.PartnerProfile, error) {
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

// GetByID retrieves a partner profile by ID
func (r *PartnerProfileRepository) GetByID(ctx context.Context, id int64) (*partner_profile.PartnerProfile, error) {
	var p partner_profile.PartnerProfile
	err := r.db.QueryRow(ctx,
		"SELECT id, account_id, shop_name, printer_id FROM partner_profiles WHERE id = $1",
		id,
	).Scan(&p.ID, &p.AccountID, &p.ShopName, &p.PrinterID)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("partner profile not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get partner profile: %w", err)
	}

	return &p, nil
}

// GetByAccountID retrieves a partner profile by account ID
func (r *PartnerProfileRepository) GetByAccountID(ctx context.Context, accountID int64) (*partner_profile.PartnerProfile, error) {
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

// GetByPrinterID retrieves a partner profile by printer_id (the unique agent identifier)
func (r *PartnerProfileRepository) GetByPrinterID(ctx context.Context, printerID string) (*partner_profile.PartnerProfile, error) {
	var p partner_profile.PartnerProfile
	err := r.db.QueryRow(ctx,
		"SELECT id, account_id, shop_name, printer_id FROM partner_profiles WHERE printer_id = $1",
		printerID,
	).Scan(&p.ID, &p.AccountID, &p.ShopName, &p.PrinterID)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("partner profile not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get partner profile: %w", err)
	}

	return &p, nil
}

// ShopNameResult represents a shop name result
type ShopNameResult struct {
	ShopName string
	ID       int64
}

// GetByShopName retrieves a partner profile by shop name
func (r *PartnerProfileRepository) GetByShopName(ctx context.Context, shopName string) (*partner_profile.PartnerProfile, error) {
	var p partner_profile.PartnerProfile
	err := r.db.QueryRow(ctx,
		"SELECT id, account_id, shop_name, printer_id FROM partner_profiles WHERE shop_name = $1",
		shopName,
	).Scan(&p.ID, &p.AccountID, &p.ShopName, &p.PrinterID)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("partner profile not found for shop name: %s", shopName)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get partner profile by shop name: %w", err)
	}

	return &p, nil
}

// GetPartnerIDByShopName gets the partner_profiles.id using shop_name
// print_jobs.partner_id should reference partner_profiles.id
func (r *PartnerProfileRepository) GetPartnerIDByShopName(ctx context.Context, shopName string) (int64, error) {
	var partnerID int64
	err := r.db.QueryRow(ctx,
		`SELECT id 
		 FROM partner_profiles 
		 WHERE shop_name = $1`,
		shopName,
	).Scan(&partnerID)

	if err == sql.ErrNoRows {
		return 0, fmt.Errorf("partner profile not found for shop name: %s", shopName)
	}
	if err != nil {
		return 0, fmt.Errorf("failed to get partner ID by shop name: %w", err)
	}

	return partnerID, nil
}

// GetAllShopNames retrieves all shop names from partner_profiles table
func (r *PartnerProfileRepository) GetAllShopNames(ctx context.Context) ([]ShopNameResult, error) {
	rows, err := r.db.Query(ctx,
		"SELECT id, shop_name FROM partner_profiles ORDER BY shop_name ASC",
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get shop names: %w", err)
	}
	defer rows.Close()

	var shops []ShopNameResult

	for rows.Next() {
		var shop ShopNameResult
		if err := rows.Scan(&shop.ID, &shop.ShopName); err != nil {
			return nil, fmt.Errorf("failed to scan shop name: %w", err)
		}
		shops = append(shops, shop)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating shop names: %w", err)
	}

	return shops, nil
}

