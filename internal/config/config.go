package config

import (
	"fmt"
	"os"
)

// Config holds all configuration for the application
type Config struct {
	GoogleClientID string
	Port           string
}

// LoadConfig loads configuration from environment variables
func LoadConfig() *Config {
	googleClientID := getEnv("GOOGLE_CLIENT_ID", "")
	port := getEnv("PORT", "8080")
	
	return &Config{
		GoogleClientID: googleClientID,
		Port:           port,
	}
}

// Validate checks if required configuration is present
func (c *Config) Validate() error {
	if c.GoogleClientID == "" {
		return fmt.Errorf("GOOGLE_CLIENT_ID environment variable is not set")
	}
	return nil
}

// getEnv gets an environment variable or returns a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

