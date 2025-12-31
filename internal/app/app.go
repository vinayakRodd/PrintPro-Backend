package app

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"print-pro-backend/internal/api/handlers/auth_handler"
	"print-pro-backend/internal/api/handlers/health"
	"print-pro-backend/internal/api/handlers/printer_handler"
	"print-pro-backend/internal/api/handlers/test_print"
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
	// Initialize printer handler
	printerHandler := printer_handler.NewPrinterHandler(
		a.repositories.PrinterRepository,
		a.repositories.PartnerProfileRepository,
		a.redisClient,
	)

	routes.RegisterRoutes(
		a.authHandler,
		printerHandler,
		a.services.RateLimiter,
		a.authMiddlewareFunc,
		cors.CORS,
		a.redisClient,
		a.postgresClient,
		health.CreateHealthCheck,
		health.RootHandler,
	)

	// Initialize test print handlers
	wd, err := os.Getwd()
	if err != nil {
		log.Printf("WARNING: Failed to get working directory, using current directory for test-print: %v", err)
		wd = "."
	}
	testPrintDir := filepath.Join(wd, "test-print")
	
	// Create directory if it doesn't exist
	if err := os.MkdirAll(testPrintDir, 0755); err != nil {
		log.Printf("WARNING: Failed to create test-print directory: %v", err)
	}

	printHandler := test_print.NewPrintHandler(testPrintDir)
	uploadHandler := test_print.NewUploadHandler(testPrintDir)

	// Register test print routes
	http.HandleFunc("/api/test-print/upload", cors.CORS(a.services.RateLimiter.LimitMiddleware(a.authMiddlewareFunc(uploadHandler.UploadFile))))
	http.HandleFunc("/api/test-print/list", cors.CORS(a.services.RateLimiter.LimitMiddleware(a.authMiddlewareFunc(printHandler.ListFiles))))
	http.HandleFunc("/api/test-print/printers", cors.CORS(a.services.RateLimiter.LimitMiddleware(a.authMiddlewareFunc(printHandler.ListPrinters))))
	http.HandleFunc("/api/test-print/print", cors.CORS(a.services.RateLimiter.LimitMiddleware(a.authMiddlewareFunc(printHandler.PrintFile))))
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

