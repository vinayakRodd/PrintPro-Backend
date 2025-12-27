package routes

import (
	"net/http"
	"print-pro-backend/internal/api/handlers/auth"
	"print-pro-backend/internal/middleware"
)

// RegisterAuthRoutes registers all authentication-related routes
func RegisterAuthRoutes(
	authHandler *auth.AuthHandler,
	rateLimiter *middleware.RateLimiter,
	authMiddlewareFunc func(http.HandlerFunc) http.HandlerFunc,
	corsHandler func(http.HandlerFunc) http.HandlerFunc,
) {
	// Public auth routes (no authentication required)
	http.HandleFunc("/api/auth/register", corsHandler(rateLimiter.LimitMiddleware(authHandler.Register)))
	http.HandleFunc("/api/auth/login", corsHandler(rateLimiter.LimitMiddleware(authHandler.Login)))
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
	
	// GetEmail can work with or without auth (supports forgot password flow with OTP verification)
	http.HandleFunc("/api/auth/email", corsHandler(rateLimiter.LimitMiddleware(authHandler.GetEmail)))
}

