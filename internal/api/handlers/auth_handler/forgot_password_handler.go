package auth_handler

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"print-pro-backend/internal/models"
)

// ForgotPassword handles password reset request - sends OTP to email
func (h *AuthHandler) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	log.Printf("Forgot password endpoint hit - Method: %s, URL: %s", r.Method, r.URL.Path)
	
	if r.Method != http.MethodPost {
		h.sendErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed", "Only POST method is allowed")
		return
	}

	var req models.ForgotPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("ERROR: Failed to decode request body: %v", err)
		h.sendErrorResponse(w, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

	// Validate email
	if req.Email == "" {
		log.Printf("ERROR: Email is empty")
		h.sendErrorResponse(w, http.StatusBadRequest, "Email is required", "Email field cannot be empty")
		return
	}

	// Normalize email
	email := strings.ToLower(strings.TrimSpace(req.Email))
	log.Printf("Forgot password request received")

	// Always generate and send OTP regardless of whether user exists
	// This prevents email enumeration attacks
	ctx := r.Context()
	
	log.Printf("Generating OTP")
	// Generate OTP
	otp, err := h.otpService.GenerateOTP(ctx, email)
	if err != nil {
		log.Printf("ERROR: Failed to generate OTP: %v", err)
		h.sendErrorResponse(w, http.StatusInternalServerError, "Failed to generate OTP", err.Error())
		return
	}
	log.Printf("OTP generated successfully")

	// Send OTP via email asynchronously (in background goroutine)
	// This allows us to return success immediately without waiting for SMTP
	go func(emailAddr, otpCode string) {
		log.Printf("Sending OTP via email (async)")
		if err := h.emailService.SendOTPEmail(emailAddr, otpCode); err != nil {
			log.Printf("ERROR: Failed to send OTP email asynchronously - Error: %v", err)
			// Note: We don't fail the request here since OTP is already stored in Redis
			// The user can still verify the OTP even if email fails (though unlikely)
		} else {
			log.Printf("SUCCESS: OTP email sent successfully (async)")
		}
	}(email, otp)

	log.Printf("OTP generated and email sending initiated (async)")

	// Return success response immediately (don't wait for email)
	h.sendJSONResponse(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "OTP sent successfully",
	})
}

