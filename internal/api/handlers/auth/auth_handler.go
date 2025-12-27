package auth

import (
	"print-pro-backend/internal/config"
	"print-pro-backend/internal/infrastructure"
	"print-pro-backend/internal/repositories"
	"print-pro-backend/internal/services"
)

// AuthHandler handles authentication-related requests
type AuthHandler struct {
	googleAuthService *services.GoogleAuthService
	sessionService    *services.SessionService
	emailService      *services.EmailService
	otpService        *services.OTPService
	userRepository    *repositories.UserRepository
	redisClient       *infrastructure.RedisClient
	jwtService        *services.JWTService
	config            *config.Config
}

// NewAuthHandler creates a new AuthHandler instance
func NewAuthHandler(
	googleAuthService *services.GoogleAuthService,
	sessionService *services.SessionService,
	emailService *services.EmailService,
	otpService *services.OTPService,
	userRepository *repositories.UserRepository,
	redisClient *infrastructure.RedisClient,
	jwtService *services.JWTService,
	cfg *config.Config,
) *AuthHandler {
	return &AuthHandler{
		googleAuthService: googleAuthService,
		sessionService:    sessionService,
		emailService:      emailService,
		otpService:        otpService,
		userRepository:    userRepository,
		redisClient:       redisClient,
		jwtService:        jwtService,
		config:            cfg,
	}
}

