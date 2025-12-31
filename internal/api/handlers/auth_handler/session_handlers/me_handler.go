package session_handlers

import (
	"net/http"
	"print-pro-backend/internal/middleware/auth_middleware"
	"print-pro-backend/internal/models"
)

// MeHandler handles user profile and user type requests
type MeHandler struct{}

// NewMeHandler creates a new MeHandler instance
func NewMeHandler() *MeHandler {
	return &MeHandler{}
}

// HandleGetMe returns the current authenticated user
func (h *MeHandler) HandleGetMe(w http.ResponseWriter, r *http.Request, sendJSONResponse func(http.ResponseWriter, int, interface{}), sendErrorResponse func(http.ResponseWriter, int, string, string)) {
	// Get user from context (set by auth middleware)
	user, ok := auth_middleware.GetUserFromContext(r)
	if !ok {
		sendErrorResponse(w, http.StatusUnauthorized, "Unauthorized", "User not found in session")
		return
	}

	// Return user information
	response := models.GoogleSignInResponse{
		Success: true,
		Message: "User authenticated",
		User:    user,
		Token:   "", // Don't return token in /me endpoint
	}

	sendJSONResponse(w, http.StatusOK, response)
}

// HandleCheckUserType returns the user type of the authenticated user
func (h *MeHandler) HandleCheckUserType(w http.ResponseWriter, r *http.Request, sendJSONResponse func(http.ResponseWriter, int, interface{}), sendErrorResponse func(http.ResponseWriter, int, string, string)) {
	// Only allow GET requests
	if r.Method != http.MethodGet {
		sendErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed", "Only GET method is allowed")
		return
	}

	// Get user from context (set by auth middleware)
	user, ok := auth_middleware.GetUserFromContext(r)
	if !ok {
		sendErrorResponse(w, http.StatusUnauthorized, "Unauthorized", "User not found in session")
		return
	}

	// Get user_type from user object (set by auth middleware)
	// The auth middleware sets user.UserType from JWT claims
	if user.UserType == "" {
		sendErrorResponse(w, http.StatusInternalServerError, "Internal error", "User type not found")
		return
	}

	// Return user type in the format expected by frontend
	response := map[string]interface{}{
		"success":   true,
		"user_type": user.UserType,
	}

	sendJSONResponse(w, http.StatusOK, response)
}

