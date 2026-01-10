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
	ID         int64     `db:"id" json:"id"`
	CustomerID int64     `db:"customer_id" json:"customer_id"`
	ShopID     int64     `db:"shop_id" json:"shop_id"`
	CreatedAt  time.Time `db:"created_at" json:"created_at"`
	UpdatedAt  time.Time `db:"updated_at" json:"updated_at"`
}

// ShopPreferenceWithShopName includes shop name for response
type ShopPreferenceWithShopName struct {
	ID       int64  `db:"id" json:"id"`
	ShopID   int64  `db:"shop_id" json:"shop_id"`
	ShopName string `db:"shop_name" json:"shop_name"`
}

// ShopPreferenceRepository handles database operations for shop preferences
type ShopPreferenceRepository struct {
	db *pgxpool.Pool
}

// NewShopPreferenceRepository creates a new shop preference repository
func NewShopPreferenceRepository(db *pgxpool.Pool) *ShopPreferenceRepository {
	return &ShopPreferenceRepository{db: db}
}

// GetByCustomerID retrieves a shop preference by customer ID
func (r *ShopPreferenceRepository) GetByCustomerID(ctx context.Context, customerID int64) (*ShopPreferenceWithShopName, error) {
	var pref ShopPreferenceWithShopName
	err := r.db.QueryRow(ctx,
		`SELECT sp.id, sp.shop_id, pp.shop_name 
		 FROM shop_preferences sp
		 INNER JOIN partner_profiles pp ON sp.shop_id = pp.id
		 WHERE sp.customer_id = $1`,
		customerID,
	).Scan(&pref.ID, &pref.ShopID, &pref.ShopName)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("shop preference not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get shop preference: %w", err)
	}

	return &pref, nil
}

// Upsert creates or updates a shop preference for a customer
func (r *ShopPreferenceRepository) Upsert(ctx context.Context, customerID, shopID int64) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO shop_preferences (customer_id, shop_id, updated_at)
		 VALUES ($1, $2, CURRENT_TIMESTAMP)
		 ON CONFLICT (customer_id) 
		 DO UPDATE SET shop_id = $2, updated_at = CURRENT_TIMESTAMP`,
		customerID, shopID,
	)

	if err != nil {
		return fmt.Errorf("failed to upsert shop preference: %w", err)
	}

	return nil
}

// DeleteByCustomerID deletes a shop preference by customer ID
func (r *ShopPreferenceRepository) DeleteByCustomerID(ctx context.Context, customerID int64) error {
	result, err := r.db.Exec(ctx,
		"DELETE FROM shop_preferences WHERE customer_id = $1",
		customerID,
	)

	if err != nil {
		return fmt.Errorf("failed to delete shop preference: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("shop preference not found")
	}

	return nil
}
