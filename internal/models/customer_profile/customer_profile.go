package customer_profile

import "database/sql"

// CustomerProfile represents a customer's profile information
type CustomerProfile struct {
	ID           int            `db:"id" json:"id"`
	AccountID    int            `db:"account_id" json:"account_id"`
	PhoneNumber  sql.NullString `db:"phone_number" json:"phone_number,omitempty"`
	WalletBalance float64       `db:"wallet_balance" json:"wallet_balance"`
}

// CreateCustomerProfileRequest represents the request to create a customer profile
type CreateCustomerProfileRequest struct {
	PhoneNumber string `json:"phone_number,omitempty"`
}

