package auth_handler

import (
	"print-pro-backend/internal/config"
	"print-pro-backend/internal/infrastructure"
	"print-pro-backend/internal/repositories"
	"print-pro-backend/internal/services/email"
	"print-pro-backend/internal/services/google_auth"
	"print-pro-backend/internal/services/jwt"
	"print-pro-backend/internal/services/otp"
	"print-pro-backend/internal/services/session"
)

// AuthHandler handles authentication-related requests
type AuthHandler struct {
	googleAuthService *google_auth.GoogleAuthService
	sessionService    *session.SessionService
	emailService      *email.EmailService
	otpService        *otp.OTPService
	userRepository    *repositories.UserRepository
	redisClient       *infrastructure.RedisClient
	jwtService        *jwt.JWTService
	config            *config.Config
}

// NewAuthHandler creates a new AuthHandler instance
func NewAuthHandler(
	googleAuthService *google_auth.GoogleAuthService,
	sessionService *session.SessionService,
	emailService *email.EmailService,
	otpService *otp.OTPService,
	userRepository *repositories.UserRepository,
	redisClient *infrastructure.RedisClient,
	jwtService *jwt.JWTService,
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

