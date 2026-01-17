package partner

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"print-pro-backend/internal/models"
	"print-pro-backend/internal/api/handlers/auth_handler/shared"
	"print-pro-backend/internal/models/account"
	"print-pro-backend/internal/repositories"
	"golang.org/x/crypto/bcrypt"
)

// LoginHandler handles partner login with email and password
type LoginHandler struct {
	accountRepository      *repositories.AccountRepository
	partnerProfileRepository *repositories.PartnerProfileRepository
	otpService             interface {
		GenerateOTP(ctx context.Context, email string) (string, error)
		MarkOTPEmailSent(ctx context.Context, email string) (bool, error)
	}
	emailService           interface {
		SendOTPEmail(email, otp string) error
	}
	tokenHelper       shared.TokenHelper
	sendErrorResponse func(http.ResponseWriter, int, string, string)
	sendJSONResponse  func(http.ResponseWriter, int, interface{})
}

// NewLoginHandler creates a new LoginHandler instance
func NewLoginHandler(
	accountRepository *repositories.AccountRepository,
	partnerProfileRepository *repositories.PartnerProfileRepository,
	otpService interface {
		GenerateOTP(ctx context.Context, email string) (string, error)
		MarkOTPEmailSent(ctx context.Context, email string) (bool, error)
	},
	emailService interface {
		SendOTPEmail(email, otp string) error
	},
	tokenHelper shared.TokenHelper,
	sendErrorResponse func(http.ResponseWriter, int, string, string),
	sendJSONResponse func(http.ResponseWriter, int, interface{}),
) *LoginHandler {
	return &LoginHandler{
		accountRepository:      accountRepository,
		partnerProfileRepository: partnerProfileRepository,
		otpService:             otpService,
		emailService:           emailService,
		tokenHelper:            tokenHelper,
		sendErrorResponse:      sendErrorResponse,
		sendJSONResponse:       sendJSONResponse,
	}
}

// HandleLogin handles partner login with email and password
// Only allows accounts with user_type = "partner" to login
func (h *LoginHandler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	log.Printf("LoginPartner endpoint hit - Method: %s, URL: %s", r.Method, r.URL.Path)

	if r.Method != http.MethodPost {
		h.sendErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed", "Only POST method is allowed")
		return
	}

	var req models.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("ERROR: Failed to decode request body: %v", err)
		h.sendErrorResponse(w, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

	// Normalize and validate email
	email := strings.ToLower(strings.TrimSpace(req.Email))
	if email == "" {
		log.Printf("ERROR: Email is empty")
		h.sendErrorResponse(w, http.StatusBadRequest, "Email is required", "Email field cannot be empty")
		return
	}

	// Trim password (but don't lowercase it - passwords are case-sensitive)
	password := strings.TrimSpace(req.Password)
	if password == "" {
		log.Printf("ERROR: Password is empty")
		h.sendErrorResponse(w, http.StatusBadRequest, "Password is required", "Password field cannot be empty")
		return
	}

	ctx := r.Context()

	// Query accounts table to get account with user_type
	log.Printf("Fetching account from database for partner login")
	accountRecord, err := h.accountRepository.GetByEmail(ctx, email)
	if err != nil {
		log.Printf("ERROR: Account not found - Error: %v", err)
		h.sendErrorResponse(w, http.StatusUnauthorized, "Invalid credentials", "Invalid email or password")
		return
	}

	log.Printf("Account found - UserType: %s", accountRecord.UserType)

	// VALIDATION: Only allow partners to login through this endpoint
	if accountRecord.UserType != account.UserTypePartner {
		log.Printf("ERROR: Attempted partner login with non-partner account")
		h.sendErrorResponse(w, http.StatusForbidden, "Invalid account type", "This email is registered as a customer. Please use the customer login.")
		return
	}

	// Check if account is OAuth user (can't login with password)
	if accountRecord.PasswordHash == "oauth_google" {
		log.Printf("ERROR: Attempted password login for OAuth user")
		h.sendErrorResponse(w, http.StatusBadRequest, "Invalid login method", "This partner account uses Google Sign-In. Please use Google to sign in.")
		return
	}

	// Trim any whitespace from the password hash stored in database
	passwordHashFromDB := strings.TrimSpace(accountRecord.PasswordHash)
	
	// Validate password hash format (bcrypt hashes start with $2a$, $2b$, or $2y$)
	if len(passwordHashFromDB) < 10 || !strings.HasPrefix(passwordHashFromDB, "$2") {
		log.Printf("ERROR: Invalid password hash format in database")
		h.sendErrorResponse(w, http.StatusInternalServerError, "Invalid password format", "Password format is invalid. Please reset your password.")
		return
	}
	
	// Verify password using bcrypt.CompareHashAndPassword
	if err := bcrypt.CompareHashAndPassword([]byte(passwordHashFromDB), []byte(password)); err != nil {
		log.Printf("ERROR: Password comparison failed - Error: %v", err)
		h.sendErrorResponse(w, http.StatusUnauthorized, "Invalid credentials", "Invalid email or password")
		return
	}

	// Check partner_profiles status - only authorized partners (status = true) can proceed
	log.Printf("Checking partner profile status")
	partnerProfile, err := h.partnerProfileRepository.GetByAccountEmail(ctx, email)
	if err != nil {
		log.Printf("ERROR: Partner profile not found - Error: %v", err)
		h.sendErrorResponse(w, http.StatusUnauthorized, "Unauthorized", "Partner profile not found. Please contact administrator.")
		return
	}

	// Check if partner status is true (authorized)
	if !partnerProfile.Status {
		log.Printf("ERROR: Partner account not authorized - Status: false")
		h.sendErrorResponse(w, http.StatusUnauthorized, "Unauthorized", "Your partner account is not authorized. Please contact administrator.")
		return
	}

	log.Printf("SUCCESS: Partner profile verified - Status: true, Shop: %s", partnerProfile.ShopName)

	// Generate and send OTP for printer agent connection (only if status is true)
	// GenerateOTP now checks for existing OTP and reuses it to prevent duplicate OTP generation
	// We also check if email was already sent to prevent duplicate emails
	log.Printf("Generating OTP for partner printer agent connection")
	otpCode, err := h.otpService.GenerateOTP(ctx, email)
	if err != nil {
		log.Printf("ERROR: Failed to generate OTP - Error: %v", err)
		h.sendErrorResponse(w, http.StatusInternalServerError, "Failed to generate OTP", "Failed to generate OTP. Please try again.")
		return
	}

	// Check if email was already sent for this OTP to prevent duplicate emails
	// This handles the case where login endpoint is called multiple times
	emailAlreadySent, err := h.otpService.MarkOTPEmailSent(ctx, email)
	if err != nil {
		log.Printf("WARNING: Failed to check email send status (non-fatal): %v", err)
		// Continue - worst case is duplicate email
		emailAlreadySent = false
	}

	// Send OTP via email asynchronously (in background goroutine)
	// Only send if email hasn't been sent recently (within 2 minutes)
	if !emailAlreadySent {
		go func(emailAddr, otpCode string) {
			log.Printf("Sending OTP via email to partner (async)")
			if err := h.emailService.SendOTPEmail(emailAddr, otpCode); err != nil {
				log.Printf("ERROR: Failed to send OTP email asynchronously - Error: %v", err)
				// Note: We don't fail the request here since OTP is already stored in Redis
			} else {
				log.Printf("SUCCESS: OTP email sent successfully to partner (async)")
			}
		}(email, otpCode)
	} else {
		log.Printf("Skipping email send - email was already sent recently for this OTP")
	}

	log.Printf("SUCCESS: Partner authenticated successfully - OTP ready for email")

	// Convert account to auth user model
	authUser := &models.User{
		ID:        accountRecord.Email, // Use email as ID
		Email:     accountRecord.Email,
		Name:      accountRecord.Email, // Will be updated based on profile if needed
		Username:  accountRecord.Username, // Username from accounts table
		UserType:  accountRecord.UserType, // Set user_type from account (should be "partner")
		Provider:  "email",
		CreatedAt: accountRecord.CreatedAt,
		UpdatedAt: accountRecord.CreatedAt,
	}

	// Generate tokens and set cookies (include user_type in JWT)
	accessToken, refreshToken, err := h.tokenHelper.GenerateTokensAndSetCookies(w, authUser.ID, authUser.Email, accountRecord.UserType, ctx)
	if err != nil {
		h.sendErrorResponse(w, http.StatusInternalServerError, "Failed to generate tokens", err.Error())
		return
	}

	// Return success response with both tokens (for Go agent compatibility)
	// Refresh token is also in cookie for web frontend compatibility
	response := map[string]interface{}{
		"success":       true,
		"message":      "Partner login successful",
		"user":         authUser,
		"access_token": accessToken,   // Access token in response body
		"token":        accessToken,    // Alias for backward compatibility
		"refresh_token": refreshToken, // Refresh token in response body (for Go agent)
	}

	h.sendJSONResponse(w, http.StatusOK, response)
}

