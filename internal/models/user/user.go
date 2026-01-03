package user

import "time"

// User represents a user in the database
type User struct {
	ID           int64     `db:"id" json:"id"`
	FullName     string    `db:"full_name" json:"full_name"`
	Email        string    `db:"email" json:"email"`
	PasswordHash string    `db:"password_hash" json:"-"` // Never expose password hash in JSON
	CreatedAt    time.Time `db:"created_at" json:"created_at"`
}

// CreateUserRequest represents the request to create a new user
type CreateUserRequest struct {
	FullName string `json:"full_name" binding:"required"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
}

// UpdateUserRequest represents the request to update a user
type UpdateUserRequest struct {
	FullName string `json:"full_name,omitempty"`
	Email    string `json:"email,omitempty" binding:"omitempty,email"`
}

