package auth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"print-pro-backend/internal/models"

	"golang.org/x/crypto/bcrypt"
)

// ResetPassword handles password reset with OTP verification
func (h *AuthHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	log.Printf("Reset password endpoint hit - Method: %s, URL: %s", r.Method, r.URL.Path)
	
	if r.Method != http.MethodPost {
		h.sendErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed", "Only POST method is allowed")
		return
	}

	// Read body for debugging (without logging sensitive data)
	bodyBytes := make([]byte, 0)
	if r.Body != nil {
		var err error
		bodyBytes, err = io.ReadAll(r.Body)
		if err == nil {
			log.Printf("Reset password request received")
			// Reset body for JSON decoder
			r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		}
	}

	var req models.ResetPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("Failed to decode request body: %v", err)
		h.sendErrorResponse(w, http.StatusBadRequest, "Invalid request body", fmt.Sprintf("Failed to parse JSON: %v", err))
		return
	}

	// Use new_password field (frontend sends this)
	password := strings.TrimSpace(req.NewPassword)
	if password == "" {
		// Fallback to password field for backward compatibility
		password = strings.TrimSpace(req.Password)
	}

	log.Printf("Password reset request - Password length: %d", len(password))

	// Validate email
	if req.Email == "" {
		log.Printf("ERROR: Email is empty")
		h.sendErrorResponse(w, http.StatusBadRequest, "Email is required", "Email field cannot be empty")
		return
	}

	// Validate password
	if password == "" {
		log.Printf("ERROR: Password is empty")
		h.sendErrorResponse(w, http.StatusBadRequest, "Password is required", "new_password field cannot be empty")
		return
	}

	if len(password) < 6 {
		log.Printf("ERROR: Password too short (length: %d)", len(password))
		h.sendErrorResponse(w, http.StatusBadRequest, "Password too short", "Password must be at least 6 characters")
		return
	}

	ctx := r.Context()

	// Normalize email
	email := strings.ToLower(strings.TrimSpace(req.Email))

	// Check if reset token exists (OTP was already verified)
	valid, err := h.otpService.VerifyResetToken(ctx, email)
	if err != nil || !valid {
		log.Printf("ERROR: Reset token not found or expired")
		h.sendErrorResponse(w, http.StatusUnauthorized, "OTP verification required", "Please verify your OTP first before resetting password")
		return
	}
	
	// Check if user exists
	user, err := h.userRepository.GetByEmail(ctx, email)
	if err != nil {
		log.Printf("ERROR: User not found - %v", err)
		h.sendErrorResponse(w, http.StatusNotFound, "User not found", "User with this email does not exist")
		return
	}

	// Check if new password is same as current password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err == nil {
		log.Printf("ERROR: New password is same as current password")
		h.sendErrorResponse(w, http.StatusBadRequest, "Current password is same as new password", "Please choose a different password")
		return
	}

	// Hash the new password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("ERROR: Failed to hash password - %v", err)
		h.sendErrorResponse(w, http.StatusInternalServerError, "Failed to process password", err.Error())
		return
	}

	// Update password in database
	if err := h.userRepository.UpdatePassword(ctx, email, string(hashedPassword)); err != nil {
		log.Printf("ERROR: Failed to update password in database - %v", err)
		h.sendErrorResponse(w, http.StatusInternalServerError, "Failed to update password", err.Error())
		return
	}

	// Delete reset token after successful password reset
	h.otpService.DeleteResetToken(ctx, email)

	log.Printf("SUCCESS: Password reset completed")

	// Return email in response so frontend doesn't need to call GetEmail
	h.sendJSONResponse(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Password has been reset successfully",
		"email":   email, // Include email so frontend doesn't need to call GetEmail
	})
}

