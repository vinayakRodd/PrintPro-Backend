package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"print-pro-backend/internal/models/account"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// AccountRepository handles database operations for accounts
type AccountRepository struct {
	db *pgxpool.Pool
}

// NewAccountRepository creates a new account repository
func NewAccountRepository(db *pgxpool.Pool) *AccountRepository {
	return &AccountRepository{db: db}
}

// GetByEmail retrieves an account by email address
func (r *AccountRepository) GetByEmail(ctx context.Context, email string) (*account.Account, error) {
	var a account.Account
	err := r.db.QueryRow(ctx,
		"SELECT id, email, password_hash, user_type, created_at FROM accounts WHERE email = $1",
		email,
	).Scan(&a.ID, &a.Email, &a.PasswordHash, &a.UserType, &a.CreatedAt)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("account not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get account: %w", err)
	}

	return &a, nil
}

// GetByID retrieves an account by ID
func (r *AccountRepository) GetByID(ctx context.Context, id int) (*account.Account, error) {
	var a account.Account
	err := r.db.QueryRow(ctx,
		"SELECT id, email, password_hash, user_type, created_at FROM accounts WHERE id = $1",
		id,
	).Scan(&a.ID, &a.Email, &a.PasswordHash, &a.UserType, &a.CreatedAt)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("account not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get account: %w", err)
	}

	return &a, nil
}

// Create creates a new account in the database
// Returns the created account with the generated ID
func (r *AccountRepository) Create(ctx context.Context, email, passwordHash, userType string) (*account.Account, error) {
	var a account.Account
	err := r.db.QueryRow(ctx,
		`INSERT INTO accounts (email, password_hash, user_type, created_at) 
		 VALUES ($1, $2, $3, $4) 
		 RETURNING id, email, password_hash, user_type, created_at`,
		email, passwordHash, userType, time.Now(),
	).Scan(&a.ID, &a.Email, &a.PasswordHash, &a.UserType, &a.CreatedAt)

	if err != nil {
		return nil, fmt.Errorf("failed to create account: %w", err)
	}

	return &a, nil
}

// UpdatePassword updates the account's password hash
func (r *AccountRepository) UpdatePassword(ctx context.Context, email, passwordHash string) error {
	_, err := r.db.Exec(ctx,
		"UPDATE accounts SET password_hash = $1 WHERE email = $2",
		passwordHash, email,
	)
	if err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}
	return nil
}

