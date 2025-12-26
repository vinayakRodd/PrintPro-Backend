package main

import (
	"fmt"
	"log"
	"net/http"
	"print-pro-backend/internal/api"
	"print-pro-backend/internal/config"
	"print-pro-backend/internal/services"
)

func main() {
	// Load configuration from environment variables
	cfg := config.LoadConfig()
	
	// Validate configuration
	if err := cfg.Validate(); err != nil {
		log.Fatalf("Configuration error: %v", err)
	}
	
	// Log configuration loaded (mask client ID for security)
	maskedClientID := maskString(cfg.GoogleClientID, 8)
	fmt.Printf("Configuration loaded:\n")
	fmt.Printf("  Google Client ID: %s\n", maskedClientID)
	fmt.Printf("  Port: %s\n", cfg.Port)

	// Initialize services
	googleAuthService := services.NewGoogleAuthService(cfg)

	// Initialize handlers
	authHandler := api.NewAuthHandler(googleAuthService)

	// Register routes
	http.HandleFunc("/", handler)
	http.HandleFunc("/health", healthCheck)
	http.HandleFunc("/ping", healthCheck)
	http.HandleFunc("/api/auth/google/signin", authHandler.GoogleSignIn)

	port := ":" + cfg.Port
	fmt.Printf("Server starting on port %s\n", port)
	fmt.Printf("Google Sign-In endpoint: http://localhost%s/api/auth/google/signin\n", port)
	log.Fatal(http.ListenAndServe(port, nil))
}

func handler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"message": "Print Pro Backend API", "status": "running"}`)
}

func healthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"status": "ok", "message": "Server is responding", "endpoint": "%s"}`, r.URL.Path)
}

// maskString masks a string showing only first few characters
func maskString(s string, visibleChars int) string {
	if len(s) <= visibleChars {
		return "***"
	}
	return s[:visibleChars] + "***"
}

