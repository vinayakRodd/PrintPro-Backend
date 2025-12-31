package app

import (
	"log"
	"print-pro-backend/internal/config"
	"print-pro-backend/internal/infrastructure"
)

// setupDatabase initializes PostgreSQL connection
func setupDatabase(cfg *config.Config) (*infrastructure.PostgresClient, error) {
	// Check if password is set (warn if empty, but allow it for some setups)
	if cfg.PostgresPassword == "" {
		log.Printf("Warning: POSTGRES_PASSWORD is not set. Using empty password.")
	}

	postgresConnString := cfg.BuildPostgresConnString()
	postgresClient, err := infrastructure.NewPostgresClient(postgresConnString)
	if err != nil {
		return nil, err
	}
	log.Printf("Connected to PostgreSQL database: %s", cfg.PostgresDB)
	return postgresClient, nil
}

