package app

import (
	"time"
	"print-pro-backend/internal/config"
	"print-pro-backend/internal/infrastructure"
	rate_limiter "print-pro-backend/internal/middleware/rate_limiter"
	"print-pro-backend/internal/services/email"
	"print-pro-backend/internal/services/google_auth"
	"print-pro-backend/internal/services/jwt"
	"print-pro-backend/internal/services/otp"
	"print-pro-backend/internal/services/session"
)

// Services holds all service instances
type Services struct {
	GoogleAuthService *google_auth.GoogleAuthService
	SessionService    *session.SessionService
	EmailService      *email.EmailService
	OTPService        *otp.OTPService
	JWTService        *jwt.JWTService
	RateLimiter       *rate_limiter.RateLimiter
}

// setupServices initializes all services
func setupServices(
	cfg *config.Config,
	redisClient *infrastructure.RedisClient,
	repos *Repositories,
) *Services {
	// Initialize services
	googleAuthService := google_auth.NewGoogleAuthService(
		cfg,
		repos.AccountRepository,
		repos.PartnerProfileRepository,
		repos.CustomerProfileRepository,
	)
	sessionService := session.NewSessionService(redisClient)
	emailService := email.NewEmailService(cfg)
	otpService := otp.NewOTPService(redisClient)
	jwtService := jwt.NewJWTService(cfg.JWTSecret)

	// Initialize rate limiter (100 requests per minute)
	rateLimiter := rate_limiter.NewRateLimiter(redisClient, 100, time.Minute)

	return &Services{
		GoogleAuthService: googleAuthService,
		SessionService:    sessionService,
		EmailService:      emailService,
		OTPService:        otpService,
		JWTService:        jwtService,
		RateLimiter:       rateLimiter,
	}
}

