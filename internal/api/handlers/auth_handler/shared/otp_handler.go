package shared

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"print-pro-backend/internal/services/otp"
)

// OTPHandler handles OTP verification (works for both customer and partner)
type OTPHandler struct {
	otpService      *otp.OTPService
	sendErrorResponse func(http.ResponseWriter, int, string, string)
	sendJSONResponse  func(http.ResponseWriter, int, interface{})
}

// NewOTPHandler creates a new OTPHandler instance
func NewOTPHandler(
	otpService *otp.OTPService,
	sendErrorResponse func(http.ResponseWriter, int, string, string),
	sendJSONResponse func(http.ResponseWriter, int, interface{}),
) *OTPHandler {
	return &OTPHandler{
		otpService:       otpService,
		sendErrorResponse: sendErrorResponse,
		sendJSONResponse:  sendJSONResponse,
	}
}

// HandleVerifyOTP handles OTP verification only (without password reset)
func (h *OTPHandler) HandleVerifyOTP(w http.ResponseWriter, r *http.Request) {
	log.Printf("Verify OTP endpoint hit - Method: %s, URL: %s", r.Method, r.URL.Path)
	
	if r.Method != http.MethodPost {
		h.sendErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed", "Only POST method is allowed")
		return
	}

	var req struct {
		Email string `json:"email"`
		OTP   string `json:"otp"`
	}
	
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("Failed to decode request body: %v", err)
		h.sendErrorResponse(w, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

	// Validate input
	if req.Email == "" || req.OTP == "" {
		log.Printf("Missing required fields - Email: %v, OTP: %v", req.Email != "", req.OTP != "")
		h.sendErrorResponse(w, http.StatusBadRequest, "All fields are required", "Email and OTP are required")
		return
	}

	ctx := r.Context()

	// Verify OTP (trim whitespace from email and OTP)
	email := strings.TrimSpace(req.Email)
	otpCode := strings.TrimSpace(req.OTP)
	
	log.Printf("Verifying OTP - OTP length: %d", len(otpCode))
	
	valid, err := h.otpService.VerifyOTP(ctx, email, otpCode)
	if err != nil || !valid {
		log.Printf("OTP verification failed: %v", err)
		h.sendErrorResponse(w, http.StatusUnauthorized, "Invalid or expired OTP", "The OTP is invalid or has expired. Please request a new one.")
		return
	}

	log.Printf("OTP verified successfully")

	h.sendJSONResponse(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "OTP verified successfully",
	})
}

