package app

import (
	"encoding/json"
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
	auth_middleware "print-pro-backend/internal/middleware/auth_middleware"
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
		a.services.AuthRateLimiter,    // 10 req/min for auth endpoints
		a.services.RefreshRateLimiter, // 30 req/min for refresh endpoint
		a.services.SearchRateLimiter,  // 100 req/min for search endpoints (shop listing)
		a.services.ProfileRateLimiter, // 150 req/min for profile/data endpoints (dashboards)
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
	agentHandler := partner_agent.NewAgentHandler(testPrintDir, archiveDir, a.redisClient, a.repositories.PrintJobRepository, a.repositories.JobCostRepository, a.services.CostCalculator)

	// Initialize WebSocket hub and handler (must be before PrintHandler)
	wsHub := websocket.NewHub()
	wsHandler := websocket.NewWebSocketHandler(wsHub, a.redisClient)
	
	printHandler := print_handler.NewPrintHandler(testPrintDir, agentHandler, a.repositories.PartnerProfileRepository, a.repositories.PrintJobRepository, a.repositories.JobCostRepository, a.repositories.AccountRepository, a.services.CostCalculator, wsHub)
	uploadHandler := test_print.NewUploadHandler(
		testPrintDir,
		agentHandler,
		a.repositories.PartnerProfileRepository,
		a.repositories.PrintJobRepository,
		a.repositories.JobCostRepository,
		a.services.CostCalculator,
	)

	// Register test print routes
	// Profile/Data endpoints: 150 req/min - Dashboard operations (partner/student dashboards)
	http.HandleFunc("/api/test-print/upload", cors.CORS(a.services.ProfileRateLimiter.LimitMiddleware(a.authMiddlewareFunc(uploadHandler.UploadFile))))
	http.HandleFunc("/api/test-print/list", cors.CORS(a.services.ProfileRateLimiter.LimitMiddleware(a.authMiddlewareFunc(printHandler.ListFiles)))) // Partner: list all files
	http.HandleFunc("/api/test-print/my-files", cors.CORS(a.services.ProfileRateLimiter.LimitMiddleware(a.authMiddlewareFunc(printHandler.ListCustomerFiles)))) // Customer: list their own files
	http.HandleFunc("/api/test-print/delete", cors.CORS(a.services.ProfileRateLimiter.LimitMiddleware(a.authMiddlewareFunc(printHandler.DeleteFile)))) // Customer: delete their own files
	http.HandleFunc("/api/test-print/printers", cors.CORS(a.services.ProfileRateLimiter.LimitMiddleware(a.authMiddlewareFunc(printHandler.ListPrinters))))
	http.HandleFunc("/api/test-print/print", cors.CORS(a.services.ProfileRateLimiter.LimitMiddleware(a.authMiddlewareFunc(printHandler.PrintFile))))
	http.HandleFunc("/api/test-print/queue", cors.CORS(a.services.ProfileRateLimiter.LimitMiddleware(a.authMiddlewareFunc(printHandler.QueueFile))))
	http.HandleFunc("/api/test-print/preview", cors.CORS(a.services.ProfileRateLimiter.LimitMiddleware(a.authMiddlewareFunc(printHandler.PreviewPDF))))
	http.HandleFunc("/api/test-print/edit-options", cors.CORS(a.services.ProfileRateLimiter.LimitMiddleware(a.authMiddlewareFunc(printHandler.EditPrintJobOptions))))
	http.HandleFunc("/api/test-print/dashboard/overview", cors.CORS(a.services.ProfileRateLimiter.LimitMiddleware(a.authMiddlewareFunc(printHandler.GetDashboardOverview))))

	// Register partner agent routes (optional JWT auth - backward compatible with Python agent)
	// No rate limiting for agent endpoints (internal communication)
	// OptionalAuthMiddleware allows requests without JWT (Python agent) or with JWT (Go agent)
	optionalAuth := auth_middleware.OptionalAuthMiddleware(a.services.JWTService)
	http.HandleFunc("/api/partner-agent/fetch-job", cors.CORS(optionalAuth(agentHandler.FetchJob)))
	http.HandleFunc("/api/partner-agent/confirm-print", cors.CORS(optionalAuth(agentHandler.ConfirmPrint)))
	http.HandleFunc("/api/partner-agent/confirm", cors.CORS(optionalAuth(agentHandler.ConfirmPrint))) // Keep for backward compatibility
	http.HandleFunc("/api/partner-agent/sync-printers", cors.CORS(optionalAuth(agentHandler.SyncPrinters)))
	http.HandleFunc("/api/partner-agent/reprint", cors.CORS(optionalAuth(agentHandler.Reprint))) // Reprint endpoint - clears Redis and resets status

	// Initialize shop handler
	shopHandler := shop_handler.NewShopHandler(
		a.repositories.PartnerProfileRepository,
		a.repositories.ShopPreferenceRepository,
		a.repositories.CustomerProfileRepository,
	)

	// Register shop routes - Search endpoint: 100 req/min (Shop listing page)
	// Shop listing (public - no auth required)
	http.HandleFunc("/api/shops/names", cors.CORS(a.services.SearchRateLimiter.LimitMiddleware(shopHandler.GetShopNames)))
	
	// Shop preference endpoints (protected - requires customer authentication)
	// Profile/Data endpoints: 150 req/min - Dashboard operations
	http.HandleFunc("/api/shops/preference", cors.CORS(a.services.ProfileRateLimiter.LimitMiddleware(a.authMiddlewareFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			shopHandler.GetShopPreference(w, r)
		case http.MethodPost:
			shopHandler.SetShopPreference(w, r)
		case http.MethodDelete:
			shopHandler.DeleteShopPreference(w, r)
		default:
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusMethodNotAllowed)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": "Method not allowed",
				"error":   "Only GET, POST, and DELETE methods are allowed",
			})
		}
	}))))

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

