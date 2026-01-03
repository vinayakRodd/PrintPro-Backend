package partner

import "time"

// Partner represents a partner (shop owner) in the database
type Partner struct {
	ID           int64     `db:"id" json:"id"`
	OwnerName    string    `db:"owner_name" json:"owner_name"`
	ShopName     string    `db:"shop_name" json:"shop_name"`
	Email        string    `db:"email" json:"email"`
	PasswordHash string    `db:"password_hash" json:"-"` // Never expose password hash in JSON
	ShopAddress  string    `db:"shop_address" json:"shop_address"`
	Latitude     *float64  `db:"latitude" json:"latitude,omitempty"`     // Nullable
	Longitude    *float64  `db:"longitude" json:"longitude,omitempty"`  // Nullable
	CreatedAt    time.Time `db:"created_at" json:"created_at"`
}

// CreatePartnerRequest represents the request to create a new partner
type CreatePartnerRequest struct {
	OwnerName   string   `json:"owner_name" binding:"required"`
	ShopName    string   `json:"shop_name" binding:"required"`
	Email       string   `json:"email" binding:"required,email"`
	Password    string   `json:"password" binding:"required,min=6"`
	ShopAddress string   `json:"shop_address" binding:"required"`
	Latitude    *float64 `json:"latitude,omitempty"`
	Longitude   *float64 `json:"longitude,omitempty"`
}

// UpdatePartnerRequest represents the request to update a partner
type UpdatePartnerRequest struct {
	OwnerName   string   `json:"owner_name,omitempty"`
	ShopName    string   `json:"shop_name,omitempty"`
	Email       string   `json:"email,omitempty" binding:"omitempty,email"`
	ShopAddress string   `json:"shop_address,omitempty"`
	Latitude    *float64 `json:"latitude,omitempty"`
	Longitude   *float64 `json:"longitude,omitempty"`
}

