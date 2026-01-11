package session_handlers

import (
	"context"
	"log"
	"net/http"
	"print-pro-backend/internal/config"
	"print-pro-backend/internal/services/jwt"
	"print-pro-backend/internal/services/session"
)

// TokenHelper handles token generation and cookie management
type TokenHelper struct {
	jwtService     *jwt.JWTService
	sessionService *session.SessionService
	config         *config.Config
}

// NewTokenHelper creates a new TokenHelper instance
func NewTokenHelper(jwtService *jwt.JWTService, sessionService *session.SessionService, cfg *config.Config) *TokenHelper {
	return &TokenHelper{
		jwtService:     jwtService,
		sessionService: sessionService,
		config:         cfg,
	}
}

// GenerateTokensAndSetCookies generates JWT tokens and sets refresh token cookie
// Returns accessToken and refreshToken, or error if generation fails
// SECURITY: Never log the actual token values - only log status messages
func (h *TokenHelper) GenerateTokensAndSetCookies(w http.ResponseWriter, userID, email, userType string, ctx context.Context) (string, string, error) {
	// SINGLE SESSION POLICY: For partners, invalidate any existing session
	if userType == "partner" {
		log.Printf("🔒 Partner login detected - invalidating existing sessions for single session policy")
		if err := h.sessionService.InvalidatePartnerSession(ctx, userID); err != nil {
			log.Printf("WARNING: Failed to invalidate existing partner session: %v", err)
			// Continue anyway - might be first login
		} else {
			log.Printf("✅ Existing partner session invalidated successfully")
		}
	}

	// Generate JWT access token (15 minutes expiry)
	log.Printf("🔑 Generating access token (expires in 15 minutes) for %s: %s", userType, userID)
	accessToken, err := h.jwtService.GenerateAccessToken(userID, email, userType)
	if err != nil {
		log.Printf("ERROR: Failed to generate access token - %v", err)
		return "", "", err
	}
	log.Printf("✅ Access token generated successfully for user: %s", userID)
	// SECURITY NOTE: accessToken contains sensitive data - never log it

	// Generate JWT refresh token (7 days expiry)
	log.Printf("🔑 Generating refresh token (expires in 7 days)")
	refreshToken, err := h.jwtService.GenerateRefreshToken(userID, email, userType)
	if err != nil {
		log.Printf("ERROR: Failed to generate refresh token - %v", err)
		return "", "", err
	}
	log.Printf("✅ Refresh token generated successfully for user: %s", userID)
	// SECURITY NOTE: refreshToken contains sensitive data - never log it

	// Store refresh token in Redis (for revocation)
	log.Printf("💾 Storing refresh token in Redis")
	if err := h.sessionService.StoreRefreshToken(ctx, refreshToken, userID); err != nil {
		log.Printf("ERROR: Failed to store refresh token in Redis: %v", err)
		return "", "", err
	}
	log.Printf("✅ Refresh token stored in Redis successfully")

	// SINGLE SESSION POLICY: For partners, track the active refresh token
	if userType == "partner" {
		log.Printf("💾 Setting active partner session")
		if err := h.sessionService.SetPartnerActiveRefreshToken(ctx, userID, refreshToken); err != nil {
			log.Printf("ERROR: Failed to set active partner session: %v", err)
			return "", "", err
		}
		log.Printf("✅ Active partner session set successfully")
	}

	// Set secure HTTP-only cookie for refresh token
	refreshCookie := &http.Cookie{
		Name:     "refresh_token",
		Value:    refreshToken,
		Path:     "/",
		MaxAge:   7 * 24 * 60 * 60, // 7 days
		HttpOnly: true,              // Prevents JavaScript access (XSS protection)
		Secure:   h.config.SecureCookies, // Set via config (true in production with HTTPS)
		SameSite: http.SameSiteLaxMode,   // CSRF protection
	}
	
	// Set domain if configured
	if h.config.CookieDomain != "" {
		refreshCookie.Domain = h.config.CookieDomain
	}
	
	http.SetCookie(w, refreshCookie)
	log.Printf("Refresh token cookie set in browser")

	return accessToken, refreshToken, nil
}

