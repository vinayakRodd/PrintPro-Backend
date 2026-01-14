package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ShopPreference represents a customer's shop preference
type ShopPreference struct {
	ID          int64     `db:"id" json:"id"` // Primary key (serial)
	CustomerEmail *string `db:"customer_email" json:"customer_email,omitempty"` // Nullable - customer account email
	CreatedAt   time.Time `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time `db:"updated_at" json:"updated_at"`
}

// ShopPreferenceWithShopName includes shop name for response
type ShopPreferenceWithShopName struct {
	ID           int64  `db:"id" json:"id"`
	CustomerEmail *string `db:"customer_email" json:"customer_email,omitempty"`
	ShopName     string `db:"shop_name" json:"shop_name"`
}

// ShopPreferenceRepository handles database operations for shop preferences
type ShopPreferenceRepository struct {
	db *pgxpool.Pool
}

// NewShopPreferenceRepository creates a new shop preference repository
func NewShopPreferenceRepository(db *pgxpool.Pool) *ShopPreferenceRepository {
	return &ShopPreferenceRepository{db: db}
}

// GetByAccountEmail retrieves a shop preference by customer account email
func (r *ShopPreferenceRepository) GetByAccountEmail(ctx context.Context, accountEmail string) (*ShopPreferenceWithShopName, error) {
	var pref ShopPreferenceWithShopName
	err := r.db.QueryRow(ctx,
		`SELECT sp.id, sp.customer_email, pp.shop_name 
		 FROM shop_preferences sp
		 INNER JOIN partner_profiles pp ON sp.customer_email = pp.partner_email
		 WHERE sp.customer_email = $1`,
		accountEmail,
	).Scan(&pref.ID, &pref.CustomerEmail, &pref.ShopName)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("shop preference not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get shop preference: %w", err)
	}

	return &pref, nil
}

// Upsert creates or updates a shop preference for a customer
func (r *ShopPreferenceRepository) Upsert(ctx context.Context, accountEmail string) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO shop_preferences (customer_email, updated_at)
		 VALUES ($1, CURRENT_TIMESTAMP)
		 ON CONFLICT (customer_email) 
		 DO UPDATE SET updated_at = CURRENT_TIMESTAMP`,
		accountEmail,
	)

	if err != nil {
		return fmt.Errorf("failed to upsert shop preference: %w", err)
	}

	return nil
}

// DeleteByAccountEmail deletes a shop preference by customer account email
func (r *ShopPreferenceRepository) DeleteByAccountEmail(ctx context.Context, accountEmail string) error {
	result, err := r.db.Exec(ctx,
		"DELETE FROM shop_preferences WHERE customer_email = $1",
		accountEmail,
	)

	if err != nil {
		return fmt.Errorf("failed to delete shop preference: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("shop preference not found")
	}

	return nil
}

// Deprecated methods for backward compatibility
// GetByCustomerID is deprecated - use GetByAccountEmail instead
func (r *ShopPreferenceRepository) GetByCustomerID(ctx context.Context, customerID int64) (*ShopPreferenceWithShopName, error) {
	return nil, fmt.Errorf("GetByCustomerID is deprecated, use GetByAccountEmail instead")
}

// Upsert with customerID and shopID is deprecated - use Upsert with accountEmail instead
func (r *ShopPreferenceRepository) UpsertOld(ctx context.Context, customerID, shopID int64) error {
	return fmt.Errorf("UpsertOld is deprecated, use Upsert with accountEmail instead")
}

// DeleteByCustomerID is deprecated - use DeleteByAccountEmail instead
func (r *ShopPreferenceRepository) DeleteByCustomerID(ctx context.Context, customerID int64) error {
	return fmt.Errorf("DeleteByCustomerID is deprecated, use DeleteByAccountEmail instead")
}
