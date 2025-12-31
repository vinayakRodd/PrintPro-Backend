package account

import "time"

// Account represents the central identity table
type Account struct {
	ID           int       `db:"id" json:"id"`
	Email        string    `db:"email" json:"email"`
	PasswordHash string    `db:"password_hash" json:"-"` // Never expose password hash
	UserType     string    `db:"user_type" json:"user_type"` // "partner" or "customer"
	CreatedAt    time.Time `db:"created_at" json:"created_at"`
}

// UserType constants
const (
	UserTypePartner  = "partner"
	UserTypeCustomer = "customer"
)

