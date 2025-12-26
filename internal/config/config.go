package config

import (
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
)

// Config holds all configuration for the application
type Config struct {
	GoogleClientID string
	Port           string
	SecureCookies  bool   // Set to true in production with HTTPS
	CookieDomain   string // Cookie domain (empty for localhost)
	
	// Redis configuration
	RedisAddr     string // Redis server address (e.g., localhost:6379)
	RedisPassword string // Redis password (empty if no auth)
	RedisDB       int    // Redis database number (default 0)
}

// LoadConfig loads configuration from environment variables
// It first tries to load from .env file, then falls back to system environment variables
func LoadConfig() *Config {
	// Try to load .env file explicitly
	if err := godotenv.Load(".env"); err != nil {
		log.Printf("Warning: Failed to load .env file: %v", err)
		log.Printf("Using system environment variables instead")
	}

	googleClientID := getEnv("GOOGLE_CLIENT_ID", "")
	port := getEnv("PORT", "8080")
	secureCookies := getEnv("SECURE_COOKIES", "false") == "true"
	cookieDomain := getEnv("COOKIE_DOMAIN", "")
	
	// Redis configuration
	redisAddr := getEnv("REDIS_ADDR", "localhost:6379")
	redisPassword := getEnv("REDIS_PASSWORD", "")
	redisDB := 0
	if dbStr := getEnv("REDIS_DB", "0"); dbStr != "" {
		fmt.Sscanf(dbStr, "%d", &redisDB)
	}
	
	return &Config{
		GoogleClientID: googleClientID,
		Port:           port,
		SecureCookies:  secureCookies,
		CookieDomain:   cookieDomain,
		RedisAddr:      redisAddr,
		RedisPassword:  redisPassword,
		RedisDB:        redisDB,
	}
}

// Validate checks if required configuration is present
func (c *Config) Validate() error {
	if c.GoogleClientID == "" {
		return fmt.Errorf("GOOGLE_CLIENT_ID environment variable is required for security")
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

