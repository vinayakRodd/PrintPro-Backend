package infrastructure

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresClient wraps the PostgreSQL connection pool
type PostgresClient struct {
	pool *pgxpool.Pool
}

// NewPostgresClient creates a new PostgreSQL connection pool
func NewPostgresClient(connString string) (*PostgresClient, error) {
	// Create connection pool
	pool, err := pgxpool.New(context.Background(), connString)
	if err != nil {
		return nil, fmt.Errorf("unable to create connection pool: %w", err)
	}

	// Test the connection with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &PostgresClient{pool: pool}, nil
}

// GetPool returns the underlying connection pool
func (p *PostgresClient) GetPool() *pgxpool.Pool {
	return p.pool
}

// Close closes the PostgreSQL connection pool
func (p *PostgresClient) Close() {
	p.pool.Close()
}

// Ping tests the database connection
func (p *PostgresClient) Ping(ctx context.Context) error {
	return p.pool.Ping(ctx)
}

