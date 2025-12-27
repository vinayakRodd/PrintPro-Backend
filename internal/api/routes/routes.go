package routes

import (
	"net/http"
	"print-pro-backend/internal/api/handlers/auth_handler"
	"print-pro-backend/internal/infrastructure"
	"print-pro-backend/internal/middleware/rate_limiter"
)

// RegisterRoutes registers all application routes
func RegisterRoutes(
	authHandler *auth_handler.AuthHandler,
	rateLimiter *rate_limiter.RateLimiter,
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
	
	// Register auth routes
	RegisterAuthRoutes(authHandler, rateLimiter, authMiddlewareFunc, corsHandler)
	
	// Root handler should be last (catches all unmatched routes)
	http.HandleFunc("/", corsHandler(handler))
}

