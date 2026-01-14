package routes

import (
	"net/http"
	"print-pro-backend/internal/api/handlers/auth_handler"
	"print-pro-backend/internal/api/handlers/auth_handler/customer"
	"print-pro-backend/internal/api/handlers/auth_handler/partner"
	"print-pro-backend/internal/middleware/rate_limiter"
)

// RegisterAuthRoutes registers all authentication-related routes
func RegisterAuthRoutes(
	authHandler *auth_handler.AuthHandler,
	authRateLimiter *rate_limiter.RateLimiter,    // 100 req/min for login/signup endpoints
	refreshRateLimiter *rate_limiter.RateLimiter, // 30 req/min for refresh endpoint
	profileRateLimiter *rate_limiter.RateLimiter, // 150 req/min for profile/data endpoints
	authMiddlewareFunc func(http.HandlerFunc) http.HandlerFunc,
	corsHandler func(http.HandlerFunc) http.HandlerFunc,
) {
	// Public registration routes (Identity-Profile pattern) - Auth: 5 req/min
	// Partner and customer handlers are now in their respective packages
	http.HandleFunc("/api/auth/register/partner", corsHandler(authRateLimiter.LimitMiddleware(partner.RegisterPartner(authHandler.PartnerAuthHandler))))
	http.HandleFunc("/api/auth/register/customer", corsHandler(authRateLimiter.LimitMiddleware(customer.RegisterCustomer(authHandler.CustomerAuthHandler))))
	
	// Legacy registration route (deprecated - use /register/partner or /register/customer)
	http.HandleFunc("/api/auth/register", corsHandler(authRateLimiter.LimitMiddleware(authHandler.Register)))
	
	// Public auth routes (no authentication required) - Auth: 5 req/min
	// Separate login endpoints for partners and customers (with validation)
	http.HandleFunc("/api/auth/login/partner", corsHandler(authRateLimiter.LimitMiddleware(partner.LoginPartner(authHandler.PartnerAuthHandler))))
	http.HandleFunc("/api/auth/login/customer", corsHandler(authRateLimiter.LimitMiddleware(customer.LoginCustomer(authHandler.CustomerAuthHandler))))
	// Legacy unified login endpoint (deprecated - use /login/partner or /login/customer)
	http.HandleFunc("/api/auth/login", corsHandler(authRateLimiter.LimitMiddleware(authHandler.Login)))
	
	// Separate Google Sign-In endpoints for partners and customers (with validation) - Auth: 5 req/min
	http.HandleFunc("/api/auth/google/signin/partner", corsHandler(authRateLimiter.LimitMiddleware(partner.GoogleSignInPartner(authHandler.PartnerAuthHandler))))
	http.HandleFunc("/api/auth/google/signin/customer", corsHandler(authRateLimiter.LimitMiddleware(customer.GoogleSignInCustomer(authHandler.CustomerAuthHandler))))
	// Legacy unified Google Sign-In endpoint (deprecated - use /google/signin/partner or /google/signin/customer)
	http.HandleFunc("/api/auth/google/signin", corsHandler(authRateLimiter.LimitMiddleware(authHandler.GoogleSignIn)))
	
	// Password reset flow - Auth: 5 req/min (prevents brute force)
	http.HandleFunc("/api/auth/logout", corsHandler(authRateLimiter.LimitMiddleware(authHandler.Logout)))
	http.HandleFunc("/api/auth/forgot-password", corsHandler(authRateLimiter.LimitMiddleware(authHandler.ForgotPassword)))
	http.HandleFunc("/api/auth/otp/generate", corsHandler(authRateLimiter.LimitMiddleware(authHandler.ForgotPassword))) // Alias for frontend
	http.HandleFunc("/api/auth/otp/verify", corsHandler(authRateLimiter.LimitMiddleware(authHandler.VerifyOTP))) // OTP verification only
	http.HandleFunc("/api/auth/reset-password", corsHandler(authRateLimiter.LimitMiddleware(authHandler.ResetPassword)))
	http.HandleFunc("/api/auth/password/reset", corsHandler(authRateLimiter.LimitMiddleware(authHandler.ResetPassword))) // Alias for frontend
	http.HandleFunc("/api/auth/refresh", corsHandler(refreshRateLimiter.LimitMiddleware(authHandler.RefreshToken))) // Refresh access token - 30 req/min
	
	// Protected auth routes (require authentication) - Profile/Data: 200 req/min
	http.HandleFunc("/api/auth/me", corsHandler(profileRateLimiter.LimitMiddleware(authMiddlewareFunc(authHandler.GetMe))))
	http.HandleFunc("/api/auth/partner/me", corsHandler(profileRateLimiter.LimitMiddleware(authMiddlewareFunc(authHandler.GetPartnerMe))))
	http.HandleFunc("/api/auth/user-type", corsHandler(profileRateLimiter.LimitMiddleware(authMiddlewareFunc(authHandler.CheckUserType))))
	http.HandleFunc("/api/auth/user-type/check", corsHandler(profileRateLimiter.LimitMiddleware(authMiddlewareFunc(authHandler.CheckUserType)))) // Alias for frontend
	
	// GetEmail can work with or without auth (supports forgot password flow with OTP verification)
	// Using auth rate limiter since it's part of password reset flow
	http.HandleFunc("/api/auth/email", corsHandler(authRateLimiter.LimitMiddleware(authHandler.GetEmail)))
}

