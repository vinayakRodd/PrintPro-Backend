package app

import (
	"log"
	"net/http"
	"print-pro-backend/internal/api/handlers/auth_handler"
	"print-pro-backend/internal/api/handlers/health"
	"print-pro-backend/internal/api/routes"
	"print-pro-backend/internal/config"
	"print-pro-backend/internal/infrastructure"
	"print-pro-backend/internal/middleware/cors"
)

// Application holds all application dependencies
type Application struct {
	cfg                *config.Config
	redisClient        *infrastructure.RedisClient
	postgresClient     *infrastructure.PostgresClient
	repositories       *Repositories
	services           *Services
	authMiddlewareFunc func(http.HandlerFunc) http.HandlerFunc
	authHandler        *auth_handler.AuthHandler
}

// NewApp creates and initializes a new Application instance
func NewApp(cfg *config.Config) (*Application, error) {
	// Setup Redis
	redisClient, err := setupRedis(cfg)
	if err != nil {
		return nil, err
	}

	// Setup Database
	postgresClient, err := setupDatabase(cfg)
	if err != nil {
		redisClient.Close()
		return nil, err
	}

	// Initialize repositories
	repositories := setupRepositories(postgresClient)

	// Initialize services
	services := setupServices(cfg, redisClient, repositories)

	// Initialize handlers and middleware
	authHandler, authMiddlewareFunc := setupHandlers(cfg, redisClient, postgresClient, repositories, services)

	return &Application{
		cfg:                cfg,
		redisClient:        redisClient,
		postgresClient:     postgresClient,
		repositories:       repositories,
		services:           services,
		authMiddlewareFunc: authMiddlewareFunc,
		authHandler:        authHandler,
	}, nil
}

// RegisterRoutes registers all application routes
func (a *Application) RegisterRoutes() {
	routes.RegisterRoutes(
		a.authHandler,
		a.services.RateLimiter,
		a.authMiddlewareFunc,
		cors.CORS,
		a.redisClient,
		a.postgresClient,
		health.CreateHealthCheck,
		health.RootHandler,
	)
}

// Close gracefully shuts down the application by closing Redis and PostgreSQL connections
func (a *Application) Close() error {
	var errs []error

	if a.redisClient != nil {
		if err := a.redisClient.Close(); err != nil {
			log.Printf("Failed to close Redis client: %v", err)
			errs = append(errs, err)
		}
	}

	if a.postgresClient != nil {
		a.postgresClient.Close()
	}

	if len(errs) > 0 {
		return errs[0] // Return first error
	}

	return nil
}

