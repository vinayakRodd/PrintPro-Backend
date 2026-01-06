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
	// Rate limiters for different endpoint categories
	AuthRateLimiter    *rate_limiter.RateLimiter // 5 req/min - Stop brute force attacks
	SearchRateLimiter  *rate_limiter.RateLimiter // 20 req/min - Save DB resources
	ProfileRateLimiter *rate_limiter.RateLimiter // 50 req/min - Normal UX
	// Legacy: Keep for backward compatibility (defaults to ProfileRateLimiter)
	RateLimiter *rate_limiter.RateLimiter
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

	// Initialize rate limiters for different endpoint categories
	// Auth endpoints: 5 req/min - Stop brute force attacks on login/signup
	authRateLimiter := rate_limiter.NewRateLimiter(redisClient, 5, time.Minute)
	
	// Search endpoints: 20 req/min - Save DB resources for heavy queries
	searchRateLimiter := rate_limiter.NewRateLimiter(redisClient, 20, time.Minute)
	
	// Profile/Data endpoints: 50 req/min - Normal UX for data retrieval
	profileRateLimiter := rate_limiter.NewRateLimiter(redisClient, 50, time.Minute)

	return &Services{
		GoogleAuthService: googleAuthService,
		SessionService:    sessionService,
		EmailService:      emailService,
		OTPService:        otpService,
		JWTService:        jwtService,
		AuthRateLimiter:   authRateLimiter,
		SearchRateLimiter: searchRateLimiter,
		ProfileRateLimiter: profileRateLimiter,
		RateLimiter:       profileRateLimiter, // Default to profile limiter for backward compatibility
	}
}

