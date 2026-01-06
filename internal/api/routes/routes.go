package routes

import (
	"net/http"
	"print-pro-backend/internal/api/handlers/auth_handler"
	"print-pro-backend/internal/api/handlers/printer_handler"
	"print-pro-backend/internal/infrastructure"
	"print-pro-backend/internal/middleware/rate_limiter"
)

// RegisterRoutes registers all application routes
func RegisterRoutes(
	authHandler *auth_handler.AuthHandler,
	printerHandler *printer_handler.PrinterHandler,
	authRateLimiter *rate_limiter.RateLimiter,    // 5 req/min for auth endpoints
	searchRateLimiter *rate_limiter.RateLimiter,  // 20 req/min for search endpoints
	profileRateLimiter *rate_limiter.RateLimiter, // 200 req/min for profile/data endpoints
	authMiddlewareFunc func(http.HandlerFunc) http.HandlerFunc,
	corsHandler func(http.HandlerFunc) http.HandlerFunc,
	redisClient *infrastructure.RedisClient,
	postgresClient *infrastructure.PostgresClient,
	createHealthCheck func(*infrastructure.RedisClient, *infrastructure.PostgresClient) http.HandlerFunc,
	handler http.HandlerFunc,
) {
	// Health check routes (no CORS needed, but adding for consistency)
	http.HandleFunc("/health", corsHandler(createHealthCheck(redisClient, postgresClient)))
	http.HandleFunc("/ping", corsHandler(createHealthCheck(redisClient, postgresClient)))

	// Printer sync route from agent (public endpoint - agent doesn't have auth token)
	// Using profile rate limiter (200/min) - normal data operation
	http.HandleFunc("/api/update-printers", corsHandler(profileRateLimiter.LimitMiddleware(printerHandler.UpdatePrintersHandler)))
	
	// Printer retrieval route (protected - requires partner authentication)
	// Using profile rate limiter (200/min) - normal data operation
	http.HandleFunc("/api/printers", corsHandler(profileRateLimiter.LimitMiddleware(authMiddlewareFunc(printerHandler.GetPrintersHandler))))
	
	// Register auth routes with appropriate rate limiters
	// Auth endpoints (login/signup): 5 req/min, Profile endpoints (/me, /user-type): 200 req/min
	RegisterAuthRoutes(authHandler, authRateLimiter, profileRateLimiter, authMiddlewareFunc, corsHandler)

	// Root handler should be last (catches all unmatched routes)
	http.HandleFunc("/", corsHandler(handler))
}

