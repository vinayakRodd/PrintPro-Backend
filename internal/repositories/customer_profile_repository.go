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
func (r *CustomerProfileRepository) Create(ctx context.Context, accountID int64, phoneNumber string) (*customer_profile.CustomerProfile, error) {
	var p customer_profile.CustomerProfile
	var phoneNum sql.NullString
	if phoneNumber != "" {
		phoneNum = sql.NullString{String: phoneNumber, Valid: true}
	}

	err := r.db.QueryRow(ctx,
		`INSERT INTO customer_profiles (account_id, phone_number, wallet_balance) 
		 VALUES ($1, $2, $3) 
		 RETURNING id, account_id, phone_number, wallet_balance`,
		accountID, phoneNum, 0.00,
	).Scan(&p.ID, &p.AccountID, &p.PhoneNumber, &p.WalletBalance)

	if err != nil {
		return nil, fmt.Errorf("failed to create customer profile: %w", err)
	}

	return &p, nil
}

// GetByAccountID retrieves a customer profile by account ID
func (r *CustomerProfileRepository) GetByAccountID(ctx context.Context, accountID int64) (*customer_profile.CustomerProfile, error) {
	var p customer_profile.CustomerProfile
	err := r.db.QueryRow(ctx,
		"SELECT id, account_id, phone_number, wallet_balance FROM customer_profiles WHERE account_id = $1",
		accountID,
	).Scan(&p.ID, &p.AccountID, &p.PhoneNumber, &p.WalletBalance)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("customer profile not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get customer profile: %w", err)
	}

	return &p, nil
}

