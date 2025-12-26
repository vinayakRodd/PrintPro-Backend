package api

import (
	"encoding/json"
	"log"
	"net/http"
	"print-pro-backend/internal/models"
	"print-pro-backend/internal/services"
)

// AuthHandler handles authentication-related requests
type AuthHandler struct {
	googleAuthService *services.GoogleAuthService
}

// NewAuthHandler creates a new AuthHandler instance
func NewAuthHandler(googleAuthService *services.GoogleAuthService) *AuthHandler {
	return &AuthHandler{
		googleAuthService: googleAuthService,
	}
}

// GoogleSignIn handles Google Sign-In requests
func (h *AuthHandler) GoogleSignIn(w http.ResponseWriter, r *http.Request) {
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
		log.Printf("Google token verification failed: %v", err)
		h.sendErrorResponse(w, http.StatusUnauthorized, "Token verification failed", err.Error())
		return
	}

	// Register or login user
	registeredUser, err := h.googleAuthService.RegisterOrLoginUser(r.Context(), user)
	if err != nil {
		log.Printf("User registration/login failed: %v", err)
		h.sendErrorResponse(w, http.StatusInternalServerError, "Failed to register/login user", err.Error())
		return
	}

	// Generate session token (optional)
	sessionToken, err := h.googleAuthService.GenerateSessionToken(registeredUser.ID)
	if err != nil {
		log.Printf("Session token generation failed: %v", err)
		// Continue without session token
		sessionToken = ""
	}

	// Send success response
	response := models.GoogleSignInResponse{
		Success: true,
		Message: "User authenticated successfully",
		User:    registeredUser,
		Token:   sessionToken,
	}

	h.sendJSONResponse(w, http.StatusOK, response)
}

// sendJSONResponse sends a JSON response
func (h *AuthHandler) sendJSONResponse(w http.ResponseWriter, statusCode int, data interface{}) {
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

