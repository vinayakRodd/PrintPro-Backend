package app

import (
	"net/http"
	"print-pro-backend/internal/api/handlers/auth_handler"
	"print-pro-backend/internal/config"
	"print-pro-backend/internal/infrastructure"
	"print-pro-backend/internal/middleware/auth_middleware"
)

// setupHandlers initializes all handlers and middleware
func setupHandlers(
	cfg *config.Config,
	redisClient *infrastructure.RedisClient,
	postgresClient *infrastructure.PostgresClient,
	repos *Repositories,
	services *Services,
) (*auth_handler.AuthHandler, func(http.HandlerFunc) http.HandlerFunc) {
	// Initialize auth middleware
	authMiddlewareFunc := auth_middleware.AuthMiddleware(services.SessionService, services.JWTService)

	// Initialize handlers
	authHandler := auth_handler.NewAuthHandler(
		services.GoogleAuthService,
		services.SessionService,
		services.EmailService,
		services.OTPService,
		repos.UserRepository,
		repos.AccountRepository,
		repos.PartnerProfileRepository,
		repos.CustomerProfileRepository,
		postgresClient,
		redisClient,
		services.JWTService,
		cfg,
	)

	return authHandler, authMiddlewareFunc
}

