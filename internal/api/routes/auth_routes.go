package routes

import (
	"net/http"
	"print-pro-backend/internal/api/handlers/auth_handler"
	"print-pro-backend/internal/middleware/rate_limiter"
)

// RegisterAuthRoutes registers all authentication-related routes
func RegisterAuthRoutes(
	authHandler *auth_handler.AuthHandler,
	rateLimiter *rate_limiter.RateLimiter,
	authMiddlewareFunc func(http.HandlerFunc) http.HandlerFunc,
	corsHandler func(http.HandlerFunc) http.HandlerFunc,
) {
	// Public registration routes (Identity-Profile pattern)
	http.HandleFunc("/api/auth/register/partner", corsHandler(rateLimiter.LimitMiddleware(authHandler.RegisterPartner)))
	http.HandleFunc("/api/auth/register/customer", corsHandler(rateLimiter.LimitMiddleware(authHandler.RegisterCustomer)))
	
	// Legacy registration route (deprecated - use /register/partner or /register/customer)
	http.HandleFunc("/api/auth/register", corsHandler(rateLimiter.LimitMiddleware(authHandler.Register)))
	
	// Public auth routes (no authentication required)
	// Separate login endpoints for partners and customers (with validation)
	http.HandleFunc("/api/auth/login/partner", corsHandler(rateLimiter.LimitMiddleware(authHandler.LoginPartner)))
	http.HandleFunc("/api/auth/login/customer", corsHandler(rateLimiter.LimitMiddleware(authHandler.LoginCustomer)))
	// Legacy unified login endpoint (deprecated - use /login/partner or /login/customer)
	http.HandleFunc("/api/auth/login", corsHandler(rateLimiter.LimitMiddleware(authHandler.Login)))
	
	// Separate Google Sign-In endpoints for partners and customers (with validation)
	http.HandleFunc("/api/auth/google/signin/partner", corsHandler(rateLimiter.LimitMiddleware(authHandler.GoogleSignInPartner)))
	http.HandleFunc("/api/auth/google/signin/customer", corsHandler(rateLimiter.LimitMiddleware(authHandler.GoogleSignInCustomer)))
	// Legacy unified Google Sign-In endpoint (deprecated - use /google/signin/partner or /google/signin/customer)
	http.HandleFunc("/api/auth/google/signin", corsHandler(rateLimiter.LimitMiddleware(authHandler.GoogleSignIn)))
	http.HandleFunc("/api/auth/logout", corsHandler(rateLimiter.LimitMiddleware(authHandler.Logout)))
	http.HandleFunc("/api/auth/forgot-password", corsHandler(rateLimiter.LimitMiddleware(authHandler.ForgotPassword)))
	http.HandleFunc("/api/auth/otp/generate", corsHandler(rateLimiter.LimitMiddleware(authHandler.ForgotPassword))) // Alias for frontend
	http.HandleFunc("/api/auth/otp/verify", corsHandler(rateLimiter.LimitMiddleware(authHandler.VerifyOTP))) // OTP verification only
	http.HandleFunc("/api/auth/reset-password", corsHandler(rateLimiter.LimitMiddleware(authHandler.ResetPassword)))
	http.HandleFunc("/api/auth/password/reset", corsHandler(rateLimiter.LimitMiddleware(authHandler.ResetPassword))) // Alias for frontend
	http.HandleFunc("/api/auth/refresh", corsHandler(rateLimiter.LimitMiddleware(authHandler.RefreshToken))) // Refresh access token
	
	// Protected auth routes (require authentication)
	http.HandleFunc("/api/auth/me", corsHandler(rateLimiter.LimitMiddleware(authMiddlewareFunc(authHandler.GetMe))))
	http.HandleFunc("/api/auth/user-type", corsHandler(rateLimiter.LimitMiddleware(authMiddlewareFunc(authHandler.CheckUserType))))
	http.HandleFunc("/api/auth/user-type/check", corsHandler(rateLimiter.LimitMiddleware(authMiddlewareFunc(authHandler.CheckUserType)))) // Alias for frontend
	
	// GetEmail can work with or without auth (supports forgot password flow with OTP verification)
	http.HandleFunc("/api/auth/email", corsHandler(rateLimiter.LimitMiddleware(authHandler.GetEmail)))
}

