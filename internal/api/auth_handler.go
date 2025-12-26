package api

import (
	"encoding/json"
	"log"
	"net/http"
	"print-pro-backend/internal/config"
	"print-pro-backend/internal/models"
	"print-pro-backend/internal/services"
)

// AuthHandler handles authentication-related requests
type AuthHandler struct {
	googleAuthService *services.GoogleAuthService
	config            *config.Config
}

// NewAuthHandler creates a new AuthHandler instance
func NewAuthHandler(googleAuthService *services.GoogleAuthService, cfg *config.Config) *AuthHandler {
	return &AuthHandler{
		googleAuthService: googleAuthService,
		config:            cfg,
	}
}

// GoogleSignIn handles Google Sign-In requests
func (h *AuthHandler) GoogleSignIn(w http.ResponseWriter, r *http.Request) {
	// Set CORS headers with specific origin (required when credentials are used)
	origin := r.Header.Get("Origin")
	if origin == "http://localhost:3000" || origin == "http://localhost:3001" || origin == "http://127.0.0.1:3000" {
		w.Header().Set("Access-Control-Allow-Origin", origin)
	} else if origin != "" {
		// Allow the origin from the request (for development)
		w.Header().Set("Access-Control-Allow-Origin", origin)
	}
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
	w.Header().Set("Access-Control-Allow-Credentials", "true")
	
	// Handle preflight OPTIONS request
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	
	// Only allow POST requests
	if r.Method != http.MethodPost {
		h.sendErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed", "Only POST method is allowed")
		return
	}

	// Parse request body
	var req models.GoogleSignInRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendErrorResponse(w, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

	// Validate token
	if req.Token == "" {
		h.sendErrorResponse(w, http.StatusBadRequest, "Token is required", "Token field cannot be empty")
		return
	}

	// Verify Google token
	user, err := h.googleAuthService.VerifyGoogleToken(r.Context(), req.Token)
	if err != nil {
		h.sendErrorResponse(w, http.StatusUnauthorized, "Token verification failed", err.Error())
		return
	}

	// Register or login user
	registeredUser, err := h.googleAuthService.RegisterOrLoginUser(r.Context(), user)
	if err != nil {
		h.sendErrorResponse(w, http.StatusInternalServerError, "Failed to register/login user", err.Error())
		return
	}

	// Generate session token
	sessionToken, err := h.googleAuthService.GenerateSessionToken(registeredUser.ID)
	if err != nil {
		h.sendErrorResponse(w, http.StatusInternalServerError, "Failed to generate session token", err.Error())
		return
	}

	// Set secure HTTP-only cookie for the session token
	cookie := &http.Cookie{
		Name:     "session_token",
		Value:    sessionToken,
		Path:     "/",
		MaxAge:   7 * 24 * 60 * 60, // 7 days
		HttpOnly: true,              // Prevents JavaScript access (XSS protection)
		Secure:   h.config.SecureCookies, // Set via config (true in production with HTTPS)
		SameSite: http.SameSiteLaxMode,   // CSRF protection
	}
	
	// Set domain if configured
	if h.config.CookieDomain != "" {
		cookie.Domain = h.config.CookieDomain
	}
	
	http.SetCookie(w, cookie)
	log.Printf("Cookie set in browser")

	// Send success response (token also in response for frontend flexibility)
	response := models.GoogleSignInResponse{
		Success: true,
		Message: "User authenticated successfully",
		User:    registeredUser,
		Token:   sessionToken, // Still include in response if frontend needs it
	}

	h.sendJSONResponse(w, http.StatusOK, response)
}

// sendJSONResponse sends a JSON response
func (h *AuthHandler) sendJSONResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	// CORS headers are already set in GoogleSignIn handler
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("Failed to encode JSON response: %v", err)
	}
}

// sendErrorResponse sends an error response
func (h *AuthHandler) sendErrorResponse(w http.ResponseWriter, statusCode int, message, errorDetail string) {
	response := models.ErrorResponse{
		Success: false,
		Message: message,
		Error:   errorDetail,
	}
	h.sendJSONResponse(w, statusCode, response)
}

