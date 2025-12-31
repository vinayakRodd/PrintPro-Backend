package session_handlers

import (
	"log"
	"net/http"
	"print-pro-backend/internal/services/jwt"
	"print-pro-backend/internal/services/session"
)

// RefreshTokenHandler handles refresh token requests
type RefreshTokenHandler struct {
	jwtService     *jwt.JWTService
	sessionService *session.SessionService
}

// NewRefreshTokenHandler creates a new RefreshTokenHandler instance
func NewRefreshTokenHandler(jwtService *jwt.JWTService, sessionService *session.SessionService) *RefreshTokenHandler {
	return &RefreshTokenHandler{
		jwtService:     jwtService,
		sessionService: sessionService,
	}
}

// HandleRefreshToken handles refresh token requests to get a new access token
func (h *RefreshTokenHandler) HandleRefreshToken(w http.ResponseWriter, r *http.Request, sendJSONResponse func(http.ResponseWriter, int, interface{}), sendErrorResponse func(http.ResponseWriter, int, string, string)) {
	log.Printf("🔄 REFRESH: Refresh token endpoint hit - Method: %s, URL: %s", r.Method, r.URL.Path)

	if r.Method != http.MethodPost {
		sendErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed", "Only POST method is allowed")
		return
	}

	// Get refresh token from cookie
	cookie, err := r.Cookie("refresh_token")
	if err != nil || cookie.Value == "" {
		log.Printf("❌ REFRESH: Refresh token cookie not found")
		sendErrorResponse(w, http.StatusUnauthorized, "Unauthorized", "Refresh token not found")
		return
	}

	refreshToken := cookie.Value
	ctx := r.Context()
	log.Printf("🔍 REFRESH: Refresh token found in cookie, validating...")

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

	// Generate new access token (use user_type from claims)
	log.Printf("🔑 REFRESH: Generating new access token (expires in 15 minutes)")
	newAccessToken, err := h.jwtService.GenerateAccessToken(claims.UserID, claims.Email, claims.UserType)
	if err != nil {
		log.Printf("ERROR: Failed to generate new access token - %v", err)
		sendErrorResponse(w, http.StatusInternalServerError, "Failed to generate access token", err.Error())
		return
	}

	log.Printf("✅ REFRESH: New access token generated successfully for user: %s", claims.UserID)
	log.Printf("🎉 REFRESH: Token refresh completed - user can continue without re-login")

	// Return new access token
	sendJSONResponse(w, http.StatusOK, map[string]interface{}{
		"success":      true,
		"access_token": newAccessToken,
		"message":      "Access token refreshed successfully",
	})
}

