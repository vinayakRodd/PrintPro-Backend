package models

import "time"

// GoogleSignInRequest represents the request body for Google Sign-In
type GoogleSignInRequest struct {
	Token string `json:"token" binding:"required"` // Google ID token
	// Note: user_type is NOT in the request - backend detects it automatically from accounts table
}

// User represents a user in the system
type User struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	Username  *string   `json:"username,omitempty"` // Username from accounts table
	Picture   string    `json:"picture,omitempty"`
	Provider  string    `json:"provider"` // e.g., "google"
	UserType  string    `json:"user_type"` // "partner" or "customer"
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

// ForgotPasswordRequest represents the request to initiate password reset
type ForgotPasswordRequest struct {
	Email string `json:"email" binding:"required,email"`
}

// ResetPasswordRequest represents the request to reset password (OTP already verified)
type ResetPasswordRequest struct {
	Email       string `json:"email"`        // Required: email address
	Password    string `json:"password"`    // Optional: for backward compatibility
	NewPassword string `json:"new_password"` // Optional: frontend uses this field (either password or new_password must be provided)
}

// RegisterRequest represents the request to register a new user (deprecated - use RegisterPartnerRequest or RegisterCustomerRequest)
type RegisterRequest struct {
	FullName string `json:"full_name"` // Required: user's full name
	Email    string `json:"email"`      // Required: user's email address
	Password string `json:"password"`   // Required: user's password (min 6 characters)
}

// RegisterPartnerRequest represents the request to register a new partner
type RegisterPartnerRequest struct {
	Email     string `json:"email"`      // Required: partner's email address
	Password  string `json:"password"`   // Required: partner's password (min 6 characters)
	ShopName  string `json:"shop_name"`  // Required: name of the print shop
	PrinterID string `json:"printer_id,omitempty"` // Optional: unique ID for the Python Agent
	Username  string `json:"username,omitempty"`   // Optional: username for the account
}

// RegisterCustomerRequest represents the request to register a new customer
type RegisterCustomerRequest struct {
	Email    string `json:"email"`    // Required: customer's email address
	Password string `json:"password"` // Required: customer's password (min 6 characters)
	Username string `json:"username,omitempty"` // Optional: username for the account
}

// LoginRequest represents the request to login with email and password
type LoginRequest struct {
	Email    string `json:"email"`    // Required: user's email address
	Password string `json:"password"`  // Required: user's password
}

