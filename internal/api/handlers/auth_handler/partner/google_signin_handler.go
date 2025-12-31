package partner

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"print-pro-backend/internal/api/handlers/auth_handler/shared"
	"print-pro-backend/internal/models"
	"print-pro-backend/internal/models/account"
	"print-pro-backend/internal/services/google_auth"
)

// GoogleSignInHandler handles Google Sign-In for partners
type GoogleSignInHandler struct {
	googleAuthService *google_auth.GoogleAuthService
	tokenHelper       shared.TokenHelper
	sendErrorResponse func(http.ResponseWriter, int, string, string)
	sendJSONResponse  func(http.ResponseWriter, int, interface{})
}

// NewGoogleSignInHandler creates a new GoogleSignInHandler instance
func NewGoogleSignInHandler(
	googleAuthService *google_auth.GoogleAuthService,
	tokenHelper shared.TokenHelper,
	sendErrorResponse func(http.ResponseWriter, int, string, string),
	sendJSONResponse func(http.ResponseWriter, int, interface{}),
) *GoogleSignInHandler {
	return &GoogleSignInHandler{
		googleAuthService: googleAuthService,
		tokenHelper:        tokenHelper,
		sendErrorResponse:  sendErrorResponse,
		sendJSONResponse:   sendJSONResponse,
	}
}

// HandleGoogleSignIn handles Google Sign-In for partners only
// Only allows accounts with user_type = "partner" to login via Google
func (h *GoogleSignInHandler) HandleGoogleSignIn(w http.ResponseWriter, r *http.Request) {
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
	log.Printf("🔍 GOOGLE PARTNER: Verifying Google ID token...")
	ctx := r.Context()
	user, err := h.googleAuthService.VerifyGoogleToken(ctx, req.Token)
	if err != nil {
		log.Printf("❌ GOOGLE PARTNER: Token verification failed - %v", err)
		h.sendErrorResponse(w, http.StatusUnauthorized, "Token verification failed", err.Error())
		return
	}
	log.Printf("✅ GOOGLE PARTNER: Token verified successfully")

	// Register or login user (backend automatically detects user_type from accounts table)
	log.Printf("🔍 GOOGLE PARTNER: Registering or logging in user (auto-detecting user_type)...")
	registeredUser, err := h.googleAuthService.RegisterOrLoginUser(ctx, user)
	if err != nil {
		log.Printf("❌ GOOGLE PARTNER: Failed to register/login user - %v", err)
		// Check if error is about account not found (Closed Loop enrollment)
		if strings.Contains(err.Error(), "account not found") || strings.Contains(err.Error(), "register first") {
			h.sendErrorResponse(w, http.StatusNotFound, "Account not found", "Account not found. Please register as a partner first using email/password registration.")
			return
		}
		h.sendErrorResponse(w, http.StatusInternalServerError, "Failed to register/login user", err.Error())
		return
	}
	log.Printf("✅ GOOGLE PARTNER: User registered/logged in - ID: %s, UserType: %s", registeredUser.ID, registeredUser.UserType)

	// VALIDATION: Only allow partners to login through this endpoint
	if registeredUser.UserType != account.UserTypePartner {
		log.Printf("ERROR: Attempted partner Google Sign-In with non-partner account - UserType: %s", registeredUser.UserType)
		h.sendErrorResponse(w, http.StatusForbidden, "Invalid account type", "This email is registered as a customer. Please use the customer Google Sign-In.")
		return
	}

	// Generate tokens and set cookies
	accessToken, _, err := h.tokenHelper.GenerateTokensAndSetCookies(w, registeredUser.ID, registeredUser.Email, registeredUser.UserType, ctx)
	if err != nil {
		h.sendErrorResponse(w, http.StatusInternalServerError, "Failed to generate tokens", err.Error())
		return
	}

	// Send success response with access token (refresh token in cookie)
	response := models.GoogleSignInResponse{
		Success: true,
		Message: "Partner authenticated successfully via Google",
		User:    registeredUser,
		Token:   accessToken, // Access token in response body
	}

	h.sendJSONResponse(w, http.StatusOK, response)
}

