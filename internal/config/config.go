package config

import (
	"fmt"
	"log"
	"net/url"
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
	
	// PostgreSQL configuration
	PostgresHost     string // PostgreSQL host (e.g., localhost)
	PostgresPort     string // PostgreSQL port (e.g., 5432)
	PostgresUser     string // PostgreSQL user (e.g., postgres)
	PostgresPassword string // PostgreSQL password
	PostgresDB       string // PostgreSQL database name (e.g., printer_db)
	PostgresSSLMode  string // PostgreSQL SSL mode (e.g., disable)
	
	// Email configuration (Gmail SMTP)
	SMTPHost     string // SMTP host (e.g., smtp.gmail.com)
	SMTPPort     string // SMTP port (e.g., 587)
	SMTPUsername string // Gmail address
	SMTPPassword string // Gmail app password
	FromEmail    string // From email address
	
	// JWT configuration
	JWTSecret string // JWT secret key for signing tokens
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
	
	// PostgreSQL configuration
	postgresHost := getEnv("POSTGRES_HOST", "localhost")
	postgresPort := getEnv("POSTGRES_PORT", "5432")
	postgresUser := getEnv("POSTGRES_USER", "postgres")
	postgresPassword := getEnv("POSTGRES_PASSWORD", "")
	postgresDB := getEnv("POSTGRES_DB", "printer_db")
	postgresSSLMode := getEnv("POSTGRES_SSLMODE", "disable")
	
	// Email configuration
	smtpHost := getEnv("SMTP_HOST", "smtp.gmail.com")
	smtpPort := getEnv("SMTP_PORT", "587")
	smtpUsername := getEnv("SMTP_USERNAME", "")
	smtpPassword := getEnv("SMTP_PASSWORD", "")
	fromEmail := getEnv("FROM_EMAIL", "")
	
	// JWT configuration
	jwtSecret := getEnv("JWT_SECRET", "")
	if jwtSecret == "" {
		// Generate a random secret if not provided (for development only)
		// In production, always set JWT_SECRET in environment
		log.Printf("Warning: JWT_SECRET not set. Using default (INSECURE - only for development)")
		jwtSecret = "your-secret-key-change-in-production-min-32-chars"
	}
	
	return &Config{
		GoogleClientID: googleClientID,
		Port:           port,
		SecureCookies:  secureCookies,
		CookieDomain:   cookieDomain,
		RedisAddr:      redisAddr,
		RedisPassword:  redisPassword,
		RedisDB:        redisDB,
		PostgresHost:     postgresHost,
		PostgresPort:     postgresPort,
		PostgresUser:     postgresUser,
		PostgresPassword: postgresPassword,
		PostgresDB:       postgresDB,
		PostgresSSLMode:  postgresSSLMode,
		SMTPHost:         smtpHost,
		SMTPPort:         smtpPort,
		SMTPUsername:      smtpUsername,
		SMTPPassword:      smtpPassword,
		FromEmail:         fromEmail,
		JWTSecret:         jwtSecret,
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

// BuildPostgresConnString builds a PostgreSQL connection string from config
// Properly URL-encodes all components to handle special characters in passwords, usernames, etc.
func (c *Config) BuildPostgresConnString() string {
	// Use url.UserPassword to properly encode username and password
	userInfo := url.UserPassword(c.PostgresUser, c.PostgresPassword)
	
	// Build URL structure
	pgURL := &url.URL{
		Scheme:   "postgres",
		User:     userInfo,
		Host:     fmt.Sprintf("%s:%s", c.PostgresHost, c.PostgresPort),
		Path:     "/" + c.PostgresDB,
		RawQuery: fmt.Sprintf("sslmode=%s", url.QueryEscape(c.PostgresSSLMode)),
	}
	
	return pgURL.String()
}

