package login

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"print-pro-backend/internal/models"
	"print-pro-backend/internal/models/account"
	"print-pro-backend/internal/api/handlers/auth_handler/shared"
	"print-pro-backend/internal/repositories"
	"golang.org/x/crypto/bcrypt"
)

// LoginHandler handles user login with email and password (legacy unified endpoint - deprecated)
type LoginHandler struct {
	accountRepository *repositories.AccountRepository
	tokenHelper       shared.TokenHelper
	sendErrorResponse func(http.ResponseWriter, int, string, string)
	sendJSONResponse  func(http.ResponseWriter, int, interface{})
}

// NewLoginHandler creates a new LoginHandler instance
func NewLoginHandler(
	accountRepository *repositories.AccountRepository,
	tokenHelper shared.TokenHelper,
	sendErrorResponse func(http.ResponseWriter, int, string, string),
	sendJSONResponse func(http.ResponseWriter, int, interface{}),
) *LoginHandler {
	return &LoginHandler{
		accountRepository: accountRepository,
		tokenHelper:       tokenHelper,
		sendErrorResponse: sendErrorResponse,
		sendJSONResponse:  sendJSONResponse,
	}
}

// HandleLogin handles user login with email and password (legacy unified endpoint - deprecated)
func (h *LoginHandler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	log.Printf("Login endpoint hit (LEGACY) - Method: %s, URL: %s", r.Method, r.URL.Path)

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
	log.Printf("Fetching account from database for login (password verification required)")
	accountRecord, err := h.accountRepository.GetByEmail(ctx, email)
	if err != nil {
		log.Printf("ERROR: Account not found - Error: %v", err)
		h.sendErrorResponse(w, http.StatusUnauthorized, "Invalid credentials", "Invalid email or password")
		return
	}

	log.Printf("Account found - UserType: %s", accountRecord.UserType)

	// Check if account is OAuth user (can't login with password)
	if accountRecord.PasswordHash == "oauth_google" {
		log.Printf("ERROR: Attempted password login for OAuth user")
		if accountRecord.UserType == account.UserTypePartner {
			h.sendErrorResponse(w, http.StatusBadRequest, "Invalid login method", "This partner account uses Google Sign-In. Please use Google to sign in or use a different email for email/password login.")
		} else {
			h.sendErrorResponse(w, http.StatusBadRequest, "Invalid login method", "This account uses Google Sign-In. Please use Google to sign in.")
		}
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

	log.Printf("SUCCESS: User authenticated successfully - UserType: %s", accountRecord.UserType)

	// Convert account to auth user model
	authUser := &models.User{
		ID:        accountRecord.Email, // Use email as ID
		Email:     accountRecord.Email,
		Name:      accountRecord.Email, // Will be updated based on profile if needed
		Username:  accountRecord.Username, // Username from accounts table
		UserType:  accountRecord.UserType, // Set user_type from account
		Provider:  "email",
		CreatedAt: accountRecord.CreatedAt,
		UpdatedAt: accountRecord.CreatedAt,
	}

	// Generate tokens and set cookies (include user_type in JWT)
	accessToken, _, err := h.tokenHelper.GenerateTokensAndSetCookies(w, authUser.ID, authUser.Email, accountRecord.UserType, ctx)
	if err != nil {
		h.sendErrorResponse(w, http.StatusInternalServerError, "Failed to generate tokens", err.Error())
		return
	}

	// Return success response with access token (refresh token in cookie)
	response := models.GoogleSignInResponse{
		Success: true,
		Message: "Login successful",
		User:    authUser,
		Token:   accessToken, // Access token in response body
	}

	h.sendJSONResponse(w, http.StatusOK, response)
}

