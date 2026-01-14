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
// Status is set to false by default (pending authorization)
func (r *PartnerProfileRepository) Create(ctx context.Context, accountEmail string, shopName, printerID string) (*partner_profile.PartnerProfile, error) {
	var p partner_profile.PartnerProfile
	err := r.db.QueryRow(ctx,
		`INSERT INTO partner_profiles (partner_email, shop_name, printer_id, status) 
		 VALUES ($1, $2, $3, false) 
		 RETURNING partner_email, shop_name, printer_id, status`,
		accountEmail, shopName, printerID,
	).Scan(&p.PartnerEmail, &p.ShopName, &p.PrinterID, &p.Status)

	if err != nil {
		return nil, fmt.Errorf("failed to create partner profile: %w", err)
	}

	return &p, nil
}

// GetByAccountEmail retrieves a partner profile by account email (primary key)
func (r *PartnerProfileRepository) GetByAccountEmail(ctx context.Context, accountEmail string) (*partner_profile.PartnerProfile, error) {
	var p partner_profile.PartnerProfile
	err := r.db.QueryRow(ctx,
		"SELECT partner_email, shop_name, printer_id, status FROM partner_profiles WHERE partner_email = $1",
		accountEmail,
	).Scan(&p.PartnerEmail, &p.ShopName, &p.PrinterID, &p.Status)

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
		"SELECT partner_email, shop_name, printer_id, status FROM partner_profiles WHERE printer_id = $1",
		printerID,
	).Scan(&p.PartnerEmail, &p.ShopName, &p.PrinterID, &p.Status)

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
	ShopName     string
	PartnerEmail string
}

// GetByShopName retrieves a partner profile by shop name
func (r *PartnerProfileRepository) GetByShopName(ctx context.Context, shopName string) (*partner_profile.PartnerProfile, error) {
	var p partner_profile.PartnerProfile
	err := r.db.QueryRow(ctx,
		"SELECT partner_email, shop_name, printer_id, status FROM partner_profiles WHERE shop_name = $1",
		shopName,
	).Scan(&p.PartnerEmail, &p.ShopName, &p.PrinterID, &p.Status)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("partner profile not found for shop name: %s", shopName)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get partner profile by shop name: %w", err)
	}

	return &p, nil
}

// GetAccountEmailByShopName gets the partner_profiles.partner_email using shop_name
func (r *PartnerProfileRepository) GetAccountEmailByShopName(ctx context.Context, shopName string) (string, error) {
	var partnerEmail string
	err := r.db.QueryRow(ctx,
		`SELECT partner_email 
		 FROM partner_profiles 
		 WHERE shop_name = $1`,
		shopName,
	).Scan(&partnerEmail)

	if err == sql.ErrNoRows {
		return "", fmt.Errorf("partner profile not found for shop name: %s", shopName)
	}
	if err != nil {
		return "", fmt.Errorf("failed to get partner email by shop name: %w", err)
	}

	return partnerEmail, nil
}

// GetAllShopNames retrieves all shop names from partner_profiles table
// Only returns shops where status = true (authorized shops visible to customers)
func (r *PartnerProfileRepository) GetAllShopNames(ctx context.Context) ([]ShopNameResult, error) {
	rows, err := r.db.Query(ctx,
		"SELECT partner_email, shop_name FROM partner_profiles WHERE status = true ORDER BY shop_name ASC",
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get shop names: %w", err)
	}
	defer rows.Close()

	var shops []ShopNameResult

	for rows.Next() {
		var shop ShopNameResult
		if err := rows.Scan(&shop.PartnerEmail, &shop.ShopName); err != nil {
			return nil, fmt.Errorf("failed to scan shop name: %w", err)
		}
		shops = append(shops, shop)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating shop names: %w", err)
	}

	return shops, nil
}

// Deprecated methods for backward compatibility - these should not be used
// GetByID is deprecated - use GetByAccountEmail instead
func (r *PartnerProfileRepository) GetByID(ctx context.Context, id int64) (*partner_profile.PartnerProfile, error) {
	return nil, fmt.Errorf("GetByID is deprecated, use GetByAccountEmail instead")
}

// GetByAccountID is deprecated - use GetByAccountEmail instead
func (r *PartnerProfileRepository) GetByAccountID(ctx context.Context, accountID int64) (*partner_profile.PartnerProfile, error) {
	return nil, fmt.Errorf("GetByAccountID is deprecated, use GetByAccountEmail instead")
}

// GetPartnerIDByShopName is deprecated - use GetAccountEmailByShopName instead
func (r *PartnerProfileRepository) GetPartnerIDByShopName(ctx context.Context, shopName string) (int64, error) {
	return 0, fmt.Errorf("GetPartnerIDByShopName is deprecated, use GetAccountEmailByShopName instead")
}

