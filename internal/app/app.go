package app

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"print-pro-backend/internal/api/handlers/auth_handler"
	"print-pro-backend/internal/api/handlers/health"
	"print-pro-backend/internal/api/handlers/printer_handler"
	"print-pro-backend/internal/api/handlers/partner_agent"
	"print-pro-backend/internal/api/handlers/shop_handler"
	"print-pro-backend/internal/api/handlers/test_print"
	print_handler "print-pro-backend/internal/api/handlers/test_print/print-handler"
	"print-pro-backend/internal/api/handlers/websocket"
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

	// Initialize partner agent handler
	archiveDir := filepath.Join(testPrintDir, "archived")
	readyDir := filepath.Join(testPrintDir, "ready")
	processingDir := filepath.Join(testPrintDir, "processing")
	os.MkdirAll(archiveDir, 0755)      // Create archive directory
	os.MkdirAll(readyDir, 0755)        // Create ready directory (only files explicitly requested for printing)
	os.MkdirAll(processingDir, 0755)  // Create processing directory
	agentHandler := partner_agent.NewAgentHandler(testPrintDir, archiveDir, a.redisClient, a.repositories.PrintJobRepository)

	// Initialize WebSocket hub and handler (must be before PrintHandler)
	wsHub := websocket.NewHub()
	wsHandler := websocket.NewWebSocketHandler(wsHub, a.redisClient)
	
	printHandler := print_handler.NewPrintHandler(testPrintDir, agentHandler, a.repositories.PartnerProfileRepository, a.repositories.PrintJobRepository, wsHub)
	uploadHandler := test_print.NewUploadHandler(
		testPrintDir,
		agentHandler,
		a.repositories.PartnerProfileRepository,
		a.repositories.PrintJobRepository,
	)

	// Register test print routes
	http.HandleFunc("/api/test-print/upload", cors.CORS(a.services.RateLimiter.LimitMiddleware(a.authMiddlewareFunc(uploadHandler.UploadFile))))
	http.HandleFunc("/api/test-print/list", cors.CORS(a.services.RateLimiter.LimitMiddleware(a.authMiddlewareFunc(printHandler.ListFiles))))
	http.HandleFunc("/api/test-print/printers", cors.CORS(a.services.RateLimiter.LimitMiddleware(a.authMiddlewareFunc(printHandler.ListPrinters))))
	http.HandleFunc("/api/test-print/print", cors.CORS(a.services.RateLimiter.LimitMiddleware(a.authMiddlewareFunc(printHandler.PrintFile))))
	http.HandleFunc("/api/test-print/queue", cors.CORS(a.services.RateLimiter.LimitMiddleware(a.authMiddlewareFunc(printHandler.QueueFile))))
	http.HandleFunc("/api/test-print/preview", cors.CORS(a.services.RateLimiter.LimitMiddleware(a.authMiddlewareFunc(printHandler.PreviewPDF))))

	// Register partner agent routes (no auth required - agent will authenticate separately)
	http.HandleFunc("/api/partner-agent/fetch-job", cors.CORS(agentHandler.FetchJob))
	http.HandleFunc("/api/partner-agent/confirm-print", cors.CORS(agentHandler.ConfirmPrint))
	http.HandleFunc("/api/partner-agent/confirm", cors.CORS(agentHandler.ConfirmPrint)) // Keep for backward compatibility
	http.HandleFunc("/api/partner-agent/sync-printers", cors.CORS(agentHandler.SyncPrinters))
	http.HandleFunc("/api/partner-agent/reprint", cors.CORS(agentHandler.Reprint)) // Reprint endpoint - clears Redis and resets status

	// Initialize shop handler
	shopHandler := shop_handler.NewShopHandler(a.repositories.PartnerProfileRepository)

	// Register shop routes (optional auth - can be public or authenticated)
	http.HandleFunc("/api/shops/names", cors.CORS(a.services.RateLimiter.LimitMiddleware(shopHandler.GetShopNames)))

	// Register WebSocket routes (hub already initialized above)
	http.HandleFunc("/ws/", cors.CORS(wsHandler.HandleWebSocket))
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

