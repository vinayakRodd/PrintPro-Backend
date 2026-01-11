package session_handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"print-pro-backend/internal/config"
	"print-pro-backend/internal/services/jwt"
	"print-pro-backend/internal/services/session"
)

// RefreshTokenHandler handles refresh token requests
type RefreshTokenHandler struct {
	jwtService     *jwt.JWTService
	sessionService *session.SessionService
	tokenHelper    *TokenHelper
	config         *config.Config
}

// NewRefreshTokenHandler creates a new RefreshTokenHandler instance
func NewRefreshTokenHandler(jwtService *jwt.JWTService, sessionService *session.SessionService, tokenHelper *TokenHelper, cfg *config.Config) *RefreshTokenHandler {
	return &RefreshTokenHandler{
		jwtService:     jwtService,
		sessionService: sessionService,
		tokenHelper:    tokenHelper,
		config:         cfg,
	}
}

// HandleRefreshToken handles refresh token requests to get a new access token
func (h *RefreshTokenHandler) HandleRefreshToken(w http.ResponseWriter, r *http.Request, sendJSONResponse func(http.ResponseWriter, int, interface{}), sendErrorResponse func(http.ResponseWriter, int, string, string)) {
	log.Printf("🔄 REFRESH: Refresh token endpoint hit - Method: %s, URL: %s", r.Method, r.URL.Path)

	if r.Method != http.MethodPost {
		sendErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed", "Only POST method is allowed")
		return
	}

	// Get refresh token from cookie OR Authorization header (for Go agent)
	// SECURITY: Never log the actual token value - only log status messages
	var refreshToken string
	ctx := r.Context()
	
	// Try Authorization header first (for Go agent)
	authHeader := r.Header.Get("Authorization")
	if authHeader != "" {
		parts := strings.Split(authHeader, " ")
		if len(parts) == 2 && parts[0] == "Bearer" {
			refreshToken = parts[1]
			log.Printf("🔍 REFRESH: Refresh token found in Authorization header, validating...")
		}
	}
	
	// Fallback to cookie (for web frontend)
	if refreshToken == "" {
	cookie, err := r.Cookie("refresh_token")
		if err == nil && cookie.Value != "" {
			refreshToken = cookie.Value
			log.Printf("🔍 REFRESH: Refresh token found in cookie, validating...")
		}
	}
	
	if refreshToken == "" {
		log.Printf("❌ REFRESH: Refresh token not found in header or cookie")
		sendErrorResponse(w, http.StatusUnauthorized, "Unauthorized", "Refresh token not found")
		return
	}

	// SECURITY NOTE: refreshToken variable contains sensitive data - never log it

	// Validate refresh token JWT
	claims, err := h.jwtService.ValidateRefreshToken(refreshToken)
	if err != nil {
		log.Printf("❌ REFRESH: Invalid refresh token - %v", err)
		sendErrorResponse(w, http.StatusUnauthorized, "Unauthorized", "Invalid or expired refresh token")
		return
	}
	log.Printf("✅ REFRESH: Refresh token JWT validated successfully for user: %s", claims.UserID)

	// Verify refresh token exists in Redis (for revocation check)
	log.Printf("🔍 REFRESH: Checking refresh token in Redis...")
	_, err = h.sessionService.ValidateRefreshToken(ctx, refreshToken)
	if err != nil {
		log.Printf("❌ REFRESH: Refresh token not found in Redis (revoked) - %v", err)
		sendErrorResponse(w, http.StatusUnauthorized, "Unauthorized", "Refresh token has been revoked")
		return
	}
	log.Printf("✅ REFRESH: Refresh token found in Redis (not revoked)")

	// SINGLE SESSION POLICY: For partners, verify this is the active session
	if claims.UserType == "partner" {
		log.Printf("🔒 REFRESH: Partner refresh detected - verifying single session policy")
		activeRefreshToken, err := h.sessionService.GetPartnerActiveRefreshToken(ctx, claims.UserID)
		if err != nil || activeRefreshToken != refreshToken {
			log.Printf("❌ REFRESH: Refresh token is not the active session for partner - invalidating")
			// Invalidate the token being used (it's not the active one)
			h.sessionService.DeleteRefreshToken(ctx, refreshToken)
			// Send specific error message for frontend to display
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": "Session expired",
				"error":   "You have been logged out because you logged in from another device. Please login again.",
				"code":    "SESSION_INVALIDATED_ANOTHER_DEVICE",
			})
			return
		}
		log.Printf("✅ REFRESH: Refresh token is the active partner session")
	}

	// TOKEN ROTATION: Invalidate old refresh token and generate new one
	log.Printf("🔄 REFRESH: Rotating refresh token (invalidating old, generating new)")
	
	// Delete old refresh token from Redis
	if err := h.sessionService.DeleteRefreshToken(ctx, refreshToken); err != nil {
		log.Printf("WARNING: Failed to delete old refresh token from Redis: %v", err)
		// Continue anyway - token will expire naturally
	}
	log.Printf("✅ REFRESH: Old refresh token invalidated in Redis")

	// Generate new access token (use user_type from claims)
	log.Printf("🔑 REFRESH: Generating new access token (expires in 15 minutes)")
	newAccessToken, err := h.jwtService.GenerateAccessToken(claims.UserID, claims.Email, claims.UserType)
	if err != nil {
		log.Printf("ERROR: Failed to generate new access token - %v", err)
		sendErrorResponse(w, http.StatusInternalServerError, "Failed to generate access token", err.Error())
		return
	}

	// Generate new refresh token (TOKEN ROTATION)
	log.Printf("🔑 REFRESH: Generating new refresh token (expires in 7 days)")
	newRefreshToken, err := h.jwtService.GenerateRefreshToken(claims.UserID, claims.Email, claims.UserType)
	if err != nil {
		log.Printf("ERROR: Failed to generate new refresh token - %v", err)
		sendErrorResponse(w, http.StatusInternalServerError, "Failed to generate refresh token", err.Error())
		return
	}

	// Store new refresh token in Redis
	if err := h.sessionService.StoreRefreshToken(ctx, newRefreshToken, claims.UserID); err != nil {
		log.Printf("ERROR: Failed to store new refresh token in Redis: %v", err)
		sendErrorResponse(w, http.StatusInternalServerError, "Failed to store refresh token", err.Error())
		return
	}
	log.Printf("✅ REFRESH: New refresh token stored in Redis")

	// SINGLE SESSION POLICY: For partners, update the active session mapping
	if claims.UserType == "partner" {
		log.Printf("💾 REFRESH: Updating active partner session")
		if err := h.sessionService.SetPartnerActiveRefreshToken(ctx, claims.UserID, newRefreshToken); err != nil {
			log.Printf("ERROR: Failed to update active partner session: %v", err)
			sendErrorResponse(w, http.StatusInternalServerError, "Failed to update session", err.Error())
			return
		}
		log.Printf("✅ REFRESH: Active partner session updated successfully")
	}

	// Set new refresh token cookie (TOKEN ROTATION)
	refreshCookie := &http.Cookie{
		Name:     "refresh_token",
		Value:    newRefreshToken,
		Path:     "/",
		MaxAge:   7 * 24 * 60 * 60, // 7 days
		HttpOnly: true,              // Prevents JavaScript access (XSS protection)
		Secure:   h.config.SecureCookies, // Set via config (true in production with HTTPS)
		SameSite: http.SameSiteLaxMode, // CSRF protection
	}
	
	// Set domain if configured
	if h.config.CookieDomain != "" {
		refreshCookie.Domain = h.config.CookieDomain
	}
	
	http.SetCookie(w, refreshCookie)
	log.Printf("✅ REFRESH: New refresh token cookie set")

	log.Printf("✅ REFRESH: New access token generated successfully for user: %s", claims.UserID)
	log.Printf("🎉 REFRESH: Token refresh completed with rotation - user can continue without re-login")
	// SECURITY NOTE: newAccessToken and newRefreshToken contain sensitive data - never log them, only return in response

	// Return new access token and new refresh token (for Go agent compatibility)
	sendJSONResponse(w, http.StatusOK, map[string]interface{}{
		"success":       true,
		"access_token":  newAccessToken,
		"refresh_token": newRefreshToken, // New rotated refresh token
		"message":       "Access token refreshed successfully",
	})
}

