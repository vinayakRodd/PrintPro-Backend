package partner_profile

// PartnerProfile represents a partner's profile information
type PartnerProfile struct {
	ID        int    `db:"id" json:"id"`
	AccountID int    `db:"account_id" json:"account_id"`
	ShopName  string `db:"shop_name" json:"shop_name"`
	PrinterID string `db:"printer_id" json:"printer_id,omitempty"`
}

// CreatePartnerProfileRequest represents the request to create a partner profile
type CreatePartnerProfileRequest struct {
	ShopName string `json:"shop_name" binding:"required"`
	PrinterID string `json:"printer_id,omitempty"`
}

