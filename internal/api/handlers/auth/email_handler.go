package auth

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"print-pro-backend/internal/middleware"
)

// GetEmail returns the email of the current authenticated user
// OR returns email if user exists and has verified OTP (for forgot password flow)
func (h *AuthHandler) GetEmail(w http.ResponseWriter, r *http.Request) {
	// First, try to get user from authenticated session
	user, ok := middleware.GetUserFromContext(r)
	if ok {
		// User is authenticated - return their email (works with GET or POST)
		h.sendJSONResponse(w, http.StatusOK, map[string]interface{}{
			"success": true,
			"email":   user.Email,
			"message": "Email retrieved successfully",
		})
		return
	}

	// If not authenticated, check if this is a forgot password flow
	// Must use POST method with email in request body
	if r.Method != http.MethodPost {
		h.sendErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed", "Only POST method is allowed")
		return
	}

	var req struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendErrorResponse(w, http.StatusBadRequest, "Invalid request body", "Email is required")
		return
	}
	
	email := req.Email

	if email == "" {
		h.sendErrorResponse(w, http.StatusBadRequest, "Email is required", "Email field cannot be empty")
		return
	}

	ctx := r.Context()
	email = strings.ToLower(strings.TrimSpace(email))

	// Check if user exists in cache or database
	dbUser, err := h.getUserFromCacheOrDB(ctx, email)
	if err != nil {
		log.Printf("ERROR: User not found for email")
		h.sendErrorResponse(w, http.StatusNotFound, "User not found", "No user found with this email address")
		return
	}

	log.Printf("User found - ID: %d", dbUser.ID)

	// Check if reset token exists (OTP was verified)
	valid, err := h.otpService.VerifyResetToken(ctx, email)
	if err != nil || !valid {
		log.Printf("ERROR: Reset token not found or expired")
		h.sendErrorResponse(w, http.StatusUnauthorized, "OTP verification required", "Please verify your OTP first before retrieving email")
		return
	}

	// User exists and OTP is verified - return email
	h.sendJSONResponse(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"email":   email,
		"message": "Email retrieved successfully",
	})
}

