package main

import (
	"fmt"
	"log"
	"net/http"
	"print-pro-backend/internal/api"
	"print-pro-backend/internal/config"
	"print-pro-backend/internal/infrastructure"
	"print-pro-backend/internal/middleware"
	"print-pro-backend/internal/services"
	"time"
)

func main() {
	// Load configuration from environment variables
	cfg := config.LoadConfig()
	
	// Validate configuration
	if err := cfg.Validate(); err != nil {
		log.Fatalf("Configuration error: %v", err)
	}

	// Initialize Redis client
	redisClient, err := infrastructure.NewRedisClient(cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB)
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	defer redisClient.Close()
	log.Printf("Connected to Redis at %s", cfg.RedisAddr)

	// Initialize services
	googleAuthService := services.NewGoogleAuthService(cfg)
	sessionService := services.NewSessionService(redisClient)

	// Initialize rate limiter (100 requests per minute)
	rateLimiter := middleware.NewRateLimiter(redisClient, 100, time.Minute)

	// Initialize auth middleware
	authMiddlewareFunc := middleware.AuthMiddleware(sessionService)

	// Initialize handlers
	authHandler := api.NewAuthHandler(googleAuthService, sessionService, cfg)

	// Register routes with CORS and rate limiting
	http.HandleFunc("/", corsHandler(handler))
	http.HandleFunc("/health", corsHandler(healthCheck))
	http.HandleFunc("/ping", corsHandler(healthCheck))
	http.HandleFunc("/api/auth/google/signin", corsHandler(rateLimiter.LimitMiddleware(authHandler.GoogleSignIn)))
	http.HandleFunc("/api/auth/logout", corsHandler(rateLimiter.LimitMiddleware(authHandler.Logout)))
	// Protected route - requires authentication
	http.HandleFunc("/api/auth/me", corsHandler(rateLimiter.LimitMiddleware(authMiddlewareFunc(authHandler.GetMe))))

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

// corsHandler adds CORS headers to responses
func corsHandler(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get the origin from the request
		origin := r.Header.Get("Origin")
		
		// Allow specific origins (for development, allow localhost:3000)
		allowedOrigins := []string{
			"http://localhost:3000",
		}
		
		// Check if origin is allowed
		allowed := false
		for _, allowedOrigin := range allowedOrigins {
			if origin == allowedOrigin {
				allowed = true
				break
			}
		}
		
		// If origin is allowed, set it; otherwise use the request origin if it's localhost
		if allowed || (origin != "" && (origin == "http://localhost:3000" || origin == "http://localhost:3001")) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		} else if origin != "" {
			// For other origins, you might want to restrict this in production
			w.Header().Set("Access-Control-Allow-Origin", origin)
		}
		
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Credentials", "true")

		// Handle preflight OPTIONS requests
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		// Call the next handler
		next(w, r)
	}
}


