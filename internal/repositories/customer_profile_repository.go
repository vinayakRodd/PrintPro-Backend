package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"print-pro-backend/internal/models/customer_profile"

	"github.com/jackc/pgx/v5/pgxpool"
)

// CustomerProfileRepository handles database operations for customer profiles
type CustomerProfileRepository struct {
	db *pgxpool.Pool
}

// NewCustomerProfileRepository creates a new customer profile repository
func NewCustomerProfileRepository(db *pgxpool.Pool) *CustomerProfileRepository {
	return &CustomerProfileRepository{db: db}
}

// Create creates a new customer profile
func (r *CustomerProfileRepository) Create(ctx context.Context, accountEmail string, phoneNumber string) (*customer_profile.CustomerProfile, error) {
	var p customer_profile.CustomerProfile
	var phoneNum sql.NullString
	if phoneNumber != "" {
		phoneNum = sql.NullString{String: phoneNumber, Valid: true}
	}

	err := r.db.QueryRow(ctx,
		`INSERT INTO customer_profiles (customer_email, phone_number, wallet_balance) 
		 VALUES ($1, $2, $3) 
		 RETURNING customer_email, phone_number, wallet_balance`,
		accountEmail, phoneNum, 0.00,
	).Scan(&p.AccountEmail, &p.PhoneNumber, &p.WalletBalance)

	if err != nil {
		return nil, fmt.Errorf("failed to create customer profile: %w", err)
	}

	return &p, nil
}

// GetByAccountEmail retrieves a customer profile by account email
func (r *CustomerProfileRepository) GetByAccountEmail(ctx context.Context, accountEmail string) (*customer_profile.CustomerProfile, error) {
	var p customer_profile.CustomerProfile
	err := r.db.QueryRow(ctx,
		"SELECT customer_email, phone_number, wallet_balance FROM customer_profiles WHERE customer_email = $1",
		accountEmail,
	).Scan(&p.AccountEmail, &p.PhoneNumber, &p.WalletBalance)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("customer profile not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get customer profile: %w", err)
	}

	return &p, nil
}

// GetByAccountID is deprecated - use GetByAccountEmail instead
// Kept for backward compatibility during migration
func (r *CustomerProfileRepository) GetByAccountID(ctx context.Context, accountID int64) (*customer_profile.CustomerProfile, error) {
	// This method should not be used anymore, but kept for compatibility
	// In practice, you'd need to look up email from account_id first
	return nil, fmt.Errorf("GetByAccountID is deprecated, use GetByAccountEmail instead")
}

