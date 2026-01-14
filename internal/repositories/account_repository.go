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
		"SELECT email, password_hash, user_type, created_at, username FROM accounts WHERE email = $1",
		email,
	).Scan(&a.Email, &a.PasswordHash, &a.UserType, &a.CreatedAt, &a.Username)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("account not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get account: %w", err)
	}

	return &a, nil
}

// Create creates a new account in the database
// Returns the created account (email is the primary key)
func (r *AccountRepository) Create(ctx context.Context, email, passwordHash, userType string, username *string) (*account.Account, error) {
	var a account.Account
	err := r.db.QueryRow(ctx,
		`INSERT INTO accounts (email, password_hash, user_type, created_at, username) 
		 VALUES ($1, $2, $3, $4, $5) 
		 RETURNING email, password_hash, user_type, created_at, username`,
		email, passwordHash, userType, time.Now(), username,
	).Scan(&a.Email, &a.PasswordHash, &a.UserType, &a.CreatedAt, &a.Username)

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

// UpdateUsername updates the account's username
func (r *AccountRepository) UpdateUsername(ctx context.Context, email string, username *string) error {
	_, err := r.db.Exec(ctx,
		"UPDATE accounts SET username = $1 WHERE email = $2",
		username, email,
	)
	if err != nil {
		return fmt.Errorf("failed to update username: %w", err)
	}
	return nil
}

