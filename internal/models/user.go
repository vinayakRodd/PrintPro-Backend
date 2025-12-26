package models

import "time"

// GoogleSignInRequest represents the request body for Google Sign-In
type GoogleSignInRequest struct {
	Token string `json:"token" binding:"required"` // Google ID token
}

// User represents a user in the system
type User struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	Picture   string    `json:"picture,omitempty"`
	Provider  string    `json:"provider"` // e.g., "google"
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// GoogleSignInResponse represents the response after successful Google Sign-In
type GoogleSignInResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	User    *User  `json:"user,omitempty"`
	Token   string `json:"token,omitempty"` // JWT token for authenticated requests (if needed)
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Error   string `json:"error,omitempty"`
}

