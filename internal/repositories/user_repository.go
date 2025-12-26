package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"print-pro-backend/internal/models/user"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// UserRepository handles database operations for users
type UserRepository struct {
	db *pgxpool.Pool
}

// NewUserRepository creates a new user repository
func NewUserRepository(db *pgxpool.Pool) *UserRepository {
	return &UserRepository{db: db}
}

// GetByEmail retrieves a user by email address
func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*user.User, error) {
	var u user.User
	err := r.db.QueryRow(ctx,
		"SELECT id, full_name, email, password_hash, created_at FROM users WHERE email = $1",
		email,
	).Scan(&u.ID, &u.FullName, &u.Email, &u.PasswordHash, &u.CreatedAt)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("user not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return &u, nil
}

// Create creates a new user in the database
func (r *UserRepository) Create(ctx context.Context, fullName, email, passwordHash string) (*user.User, error) {
	var u user.User
	err := r.db.QueryRow(ctx,
		`INSERT INTO users (full_name, email, password_hash, created_at) 
		 VALUES ($1, $2, $3, $4) 
		 RETURNING id, full_name, email, password_hash, created_at`,
		fullName, email, passwordHash, time.Now(),
	).Scan(&u.ID, &u.FullName, &u.Email, &u.PasswordHash, &u.CreatedAt)

	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return &u, nil
}

// GetByID retrieves a user by ID
func (r *UserRepository) GetByID(ctx context.Context, id int) (*user.User, error) {
	var u user.User
	err := r.db.QueryRow(ctx,
		"SELECT id, full_name, email, password_hash, created_at FROM users WHERE id = $1",
		id,
	).Scan(&u.ID, &u.FullName, &u.Email, &u.PasswordHash, &u.CreatedAt)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("user not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return &u, nil
}

// Update updates user information
func (r *UserRepository) Update(ctx context.Context, id int, fullName, email string) error {
	_, err := r.db.Exec(ctx,
		"UPDATE users SET full_name = $1, email = $2 WHERE id = $3",
		fullName, email, id,
	)
	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}
	return nil
}

// UpdatePassword updates the user's password hash
func (r *UserRepository) UpdatePassword(ctx context.Context, email, passwordHash string) error {
	_, err := r.db.Exec(ctx,
		"UPDATE users SET password_hash = $1 WHERE email = $2",
		passwordHash, email,
	)
	if err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}
	return nil
}

