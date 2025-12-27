package main

import (
	"fmt"
	"log"
	"net/http"
	"print-pro-backend/internal/api/handlers/auth_handler"
	"print-pro-backend/internal/api/handlers/health"
	"print-pro-backend/internal/api/routes"
	"print-pro-backend/internal/config"
	"print-pro-backend/internal/infrastructure"
	"print-pro-backend/internal/middleware/auth_middleware"
	"print-pro-backend/internal/middleware/cors"
	"print-pro-backend/internal/middleware/rate_limiter"
	"print-pro-backend/internal/repositories"
	"print-pro-backend/internal/services/email"
	"print-pro-backend/internal/services/google_auth"
	"print-pro-backend/internal/services/jwt"
	"print-pro-backend/internal/services/otp"
	"print-pro-backend/internal/services/session"
	"time"
)

func main() {
	// Setup application components
	cfg := setupConfig()
	redisClient := setupRedis(cfg)
	defer func() {
		if err := redisClient.Close(); err != nil {
			log.Printf("Failed to close Redis client: %v", err)
		}
	}()

	postgresClient := setupDatabase(cfg)
	defer func() {
		postgresClient.Close()
	}()

	// Initialize application services and handlers
	app := setupApplication(cfg, redisClient, postgresClient)

	// Register all routes
	setupRoutes(app, redisClient, postgresClient)

	// Start server
	startServer(cfg)
}

// setupConfig loads and validates application configuration.
func setupConfig() *config.Config {
	cfg := config.LoadConfig()
	if err := cfg.Validate(); err != nil {
		log.Fatalf("Configuration error: %v", err)
	}
	return cfg
}

// setupRedis initializes Redis connection.
func setupRedis(cfg *config.Config) *infrastructure.RedisClient {
	redisClient, err := infrastructure.NewRedisClient(cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB)
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	log.Printf("Connected to Redis at %s", cfg.RedisAddr)
	return redisClient
}

// setupDatabase initializes PostgreSQL connection.
func setupDatabase(cfg *config.Config) *infrastructure.PostgresClient {
	// Check if password is set (warn if empty, but allow it for some setups)
	if cfg.PostgresPassword == "" {
		log.Printf("Warning: POSTGRES_PASSWORD is not set. Using empty password.")
	}

	postgresConnString := cfg.BuildPostgresConnString()
	postgresClient, err := infrastructure.NewPostgresClient(postgresConnString)
	if err != nil {
		log.Fatalf("Failed to connect to PostgreSQL: %v", err)
	}
	log.Printf("Connected to PostgreSQL database: %s", cfg.PostgresDB)
	return postgresClient
}

// Application holds all application dependencies
type Application struct {
	cfg                *config.Config
	redisClient        *infrastructure.RedisClient
	postgresClient     *infrastructure.PostgresClient
	userRepository     *repositories.UserRepository
	googleAuthService  *google_auth.GoogleAuthService
	sessionService     *session.SessionService
	emailService       *email.EmailService
	otpService         *otp.OTPService
	jwtService         *jwt.JWTService
	rateLimiter        *rate_limiter.RateLimiter
	authMiddlewareFunc func(http.HandlerFunc) http.HandlerFunc
	authHandler        *auth_handler.AuthHandler
}

// setupApplication initializes all application services, repositories, and handlers.
func setupApplication(cfg *config.Config, redisClient *infrastructure.RedisClient, postgresClient *infrastructure.PostgresClient) *Application {
	// Initialize repositories
	userRepository := repositories.NewUserRepository(postgresClient.GetPool())

	// Initialize services
	googleAuthService := google_auth.NewGoogleAuthService(cfg, userRepository)
	sessionService := session.NewSessionService(redisClient)
	emailService := email.NewEmailService(cfg)
	otpService := otp.NewOTPService(redisClient)
	jwtService := jwt.NewJWTService(cfg.JWTSecret)

	// Initialize rate limiter (100 requests per minute)
	rateLimiter := rate_limiter.NewRateLimiter(redisClient, 100, time.Minute)

	// Initialize auth middleware
	authMiddlewareFunc := auth_middleware.AuthMiddleware(sessionService, jwtService)

	// Initialize handlers
	authHandler := auth_handler.NewAuthHandler(googleAuthService, sessionService, emailService, otpService, userRepository, redisClient, jwtService, cfg)

	return &Application{
		cfg:                cfg,
		redisClient:        redisClient,
		postgresClient:     postgresClient,
		userRepository:     userRepository,
		googleAuthService:  googleAuthService,
		sessionService:     sessionService,
		emailService:       emailService,
		otpService:         otpService,
		jwtService:         jwtService,
		rateLimiter:        rateLimiter,
		authMiddlewareFunc: authMiddlewareFunc,
		authHandler:        authHandler,
	}
}

// setupRoutes registers all application routes.
func setupRoutes(app *Application, redisClient *infrastructure.RedisClient, postgresClient *infrastructure.PostgresClient) {
	routes.RegisterRoutes(
		app.authHandler,
		app.rateLimiter,
		app.authMiddlewareFunc,
		cors.CORS,
		redisClient,
		postgresClient,
		health.CreateHealthCheck,
		handler,
	)
}

// startServer starts the HTTP server.
func startServer(cfg *config.Config) {
	port := ":" + cfg.Port
	fmt.Printf("Server starting on port %s\n", port)
	fmt.Printf("Google Sign-In endpoint: http://localhost%s/api/auth/google/signin\n", port)
	if err := http.ListenAndServe(port, nil); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

// handler handles root path requests
func handler(w http.ResponseWriter, r *http.Request) {
	// Only handle root path, not API routes
	if r.URL.Path != "/" {
		log.Printf("Root handler - path not found: %s", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, `{"success": false, "message": "Not Found", "error": "Endpoint not found: %s"}`, r.URL.Path)
		return
	}
	log.Printf("Root handler hit - Method: %s, URL: %s", r.Method, r.URL.Path)
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"message": "Print Pro Backend API", "status": "running"}`)
}

