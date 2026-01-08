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
	AuthRateLimiter    *rate_limiter.RateLimiter // 10 req/min - Stop brute force attacks (increased from 5)
	SearchRateLimiter  *rate_limiter.RateLimiter // 100 req/min - Shop listing (increased from 20)
	ProfileRateLimiter *rate_limiter.RateLimiter // 150 req/min - Dashboard operations (increased from 50)
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
	// Auth endpoints: 10 req/min - Stop brute force attacks on login/signup (increased from 5)
	authRateLimiter := rate_limiter.NewRateLimiter(redisClient, 10, time.Minute)
	
	// Search endpoints: 100 req/min - Shop listing page (increased from 20)
	searchRateLimiter := rate_limiter.NewRateLimiter(redisClient, 100, time.Minute)
	
	// Profile/Data endpoints: 150 req/min - Dashboard operations (partner/student dashboards) (increased from 50)
	profileRateLimiter := rate_limiter.NewRateLimiter(redisClient, 150, time.Minute)

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

