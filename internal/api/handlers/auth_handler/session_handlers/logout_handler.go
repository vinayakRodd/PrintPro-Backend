package session_handlers

import (
	"log"
	"net/http"
	"print-pro-backend/internal/config"
	"print-pro-backend/internal/models"
	"print-pro-backend/internal/services/session"
)

// LogoutHandler handles user logout
type LogoutHandler struct {
	sessionService *session.SessionService
	config         *config.Config
}

// NewLogoutHandler creates a new LogoutHandler instance
func NewLogoutHandler(sessionService *session.SessionService, cfg *config.Config) *LogoutHandler {
	return &LogoutHandler{
		sessionService: sessionService,
		config:         cfg,
	}
}

// HandleLogout handles user logout by deleting the refresh token
func (h *LogoutHandler) HandleLogout(w http.ResponseWriter, r *http.Request, sendJSONResponse func(http.ResponseWriter, int, interface{}), sendErrorResponse func(http.ResponseWriter, int, string, string)) {
	// Only allow POST requests
	if r.Method != http.MethodPost {
		sendErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed", "Only POST method is allowed")
		return
	}

	ctx := r.Context()

	// Get refresh token from cookie and delete it from Redis
	refreshCookie, err := r.Cookie("refresh_token")
	if err == nil && refreshCookie.Value != "" {
		// Delete refresh token from Redis
		if err := h.sessionService.DeleteRefreshToken(ctx, refreshCookie.Value); err != nil {
			log.Printf("Failed to delete refresh token from Redis: %v", err)
		}
	}

	// Delete the refresh token cookie by setting it with MaxAge: -1
	deleteCookie := &http.Cookie{
		Name:     "refresh_token",
		Value:    "",
		Path:     "/",
		MaxAge:   -1, // Delete the cookie
		HttpOnly: true,
		Secure:   h.config.SecureCookies,
		SameSite: http.SameSiteLaxMode,
	}
	
	// Set domain if configured
	if h.config.CookieDomain != "" {
		deleteCookie.Domain = h.config.CookieDomain
	}
	
	http.SetCookie(w, deleteCookie)
	log.Printf("Refresh token cookie deleted from browser")

	// Send success response
	response := models.GoogleSignInResponse{
		Success: true,
		Message: "Logged out successfully",
		User:    nil,
		Token:   "",
	}

	sendJSONResponse(w, http.StatusOK, response)
}

