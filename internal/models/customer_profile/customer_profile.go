package customer_profile

import "database/sql"

// CustomerProfile represents a customer's profile information
type CustomerProfile struct {
	AccountEmail string         `db:"customer_email" json:"customer_email"` // Primary key, references accounts(email)
	PhoneNumber  sql.NullString `db:"phone_number" json:"phone_number,omitempty"`
	WalletBalance float64       `db:"wallet_balance" json:"wallet_balance"`
}

// CreateCustomerProfileRequest represents the request to create a customer profile
type CreateCustomerProfileRequest struct {
	PhoneNumber string `json:"phone_number,omitempty"`
}

