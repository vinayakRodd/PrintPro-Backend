package partner_profile

// PartnerProfile represents a partner's profile information
type PartnerProfile struct {
	PartnerEmail string `db:"partner_email" json:"partner_email"` // Primary key, references accounts(email)
	ShopName     string `db:"shop_name" json:"shop_name"`
	PrinterID    string `db:"printer_id" json:"printer_id,omitempty"`
	Status       bool   `db:"status" json:"status"` // Authorization status: true = authorized (visible), false = pending (hidden)
}

// CreatePartnerProfileRequest represents the request to create a partner profile
type CreatePartnerProfileRequest struct {
	ShopName string `json:"shop_name" binding:"required"`
	PrinterID string `json:"printer_id,omitempty"`
}

