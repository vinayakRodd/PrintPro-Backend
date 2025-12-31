package main

import (
	"fmt"
	"log"
	"net/http"
	"print-pro-backend/internal/app"
	"print-pro-backend/internal/config"
)

func main() {
	// Load configuration
	cfg := config.LoadConfig()
	if err := cfg.Validate(); err != nil {
		log.Fatalf("Configuration error: %v", err)
	}

	// Initialize application
	application, err := app.NewApp(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize application: %v", err)
	}
	defer application.Close()

	// Register all routes
	application.RegisterRoutes()

	// Start server
	port := ":" + cfg.Port
	fmt.Printf("Server starting on port %s\n", port)
	fmt.Printf("Google Sign-In endpoints:\n")
	fmt.Printf("  - Partner: http://localhost%s/api/auth/google/signin/partner\n", port)
	fmt.Printf("  - Customer: http://localhost%s/api/auth/google/signin/customer\n", port)
	
	if err := http.ListenAndServe(port, nil); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
