package auth_handler

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"print-pro-backend/internal/models"
	"print-pro-backend/internal/models/account"
	"golang.org/x/crypto/bcrypt"
)

// Login handles user login with email and password (legacy unified endpoint - deprecated)
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
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

	log.Printf("Account found - ID: %d, UserType: %s", accountRecord.ID, accountRecord.UserType)

	// Check if account is OAuth user (can't login with password)
	if accountRecord.PasswordHash == "oauth_google" {
		log.Printf("ERROR: Attempted password login for OAuth user - UserType: %s", accountRecord.UserType)
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

	log.Printf("SUCCESS: Password verified - authentication successful for %s", accountRecord.UserType)

	// Convert account to auth user model
	authUser := &models.User{
		ID:        fmt.Sprintf("%d", accountRecord.ID),
		Email:     accountRecord.Email,
		Name:      accountRecord.Email, // Will be updated based on profile if needed
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

// Register handles user registration (legacy - deprecated)
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	log.Printf("Register endpoint hit (DEPRECATED) - Method: %s, URL: %s", r.Method, r.URL.Path)
	h.sendErrorResponse(w, http.StatusGone, "Deprecated", "This endpoint is deprecated. Please use /api/auth/register/partner or /api/auth/register/customer.")
}

// GoogleSignIn handles Google Sign-In requests (legacy unified endpoint - deprecated)
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
	log.Printf("🔍 GOOGLE: Verifying Google ID token...")
	ctx := r.Context()
	user, err := h.googleAuthService.VerifyGoogleToken(ctx, req.Token)
	if err != nil {
		log.Printf("❌ GOOGLE: Token verification failed - %v", err)
		h.sendErrorResponse(w, http.StatusUnauthorized, "Token verification failed", err.Error())
		return
	}
	log.Printf("✅ GOOGLE: Token verified successfully")

	// Register or login user (backend automatically detects user_type from accounts table)
	log.Printf("🔍 GOOGLE: Registering or logging in user (auto-detecting user_type)...")
	registeredUser, err := h.googleAuthService.RegisterOrLoginUser(ctx, user)
	if err != nil {
		log.Printf("❌ GOOGLE: Failed to register/login user - %v", err)
		// Check if error is about account not found (Closed Loop enrollment)
		if strings.Contains(err.Error(), "account not found") || strings.Contains(err.Error(), "register first") {
			h.sendErrorResponse(w, http.StatusNotFound, "Account not found", "Account not found. Please register as a partner or customer first using email/password registration.")
			return
		}
		h.sendErrorResponse(w, http.StatusInternalServerError, "Failed to register/login user", err.Error())
		return
	}
	log.Printf("✅ GOOGLE: User registered/logged in - ID: %s, UserType: %s", registeredUser.ID, registeredUser.UserType)

	// Generate tokens and set cookies
	finalUserType := registeredUser.UserType
	if finalUserType == "" {
		// Fallback to customer if user_type not set (for backward compatibility)
		finalUserType = account.UserTypeCustomer
	}
	accessToken, _, err := h.tokenHelper.GenerateTokensAndSetCookies(w, registeredUser.ID, registeredUser.Email, finalUserType, ctx)
	if err != nil {
		h.sendErrorResponse(w, http.StatusInternalServerError, "Failed to generate tokens", err.Error())
		return
	}

	// Send success response with access token (refresh token in cookie)
	response := models.GoogleSignInResponse{
		Success: true,
		Message: "User authenticated successfully",
		User:    registeredUser,
		Token:   accessToken, // Access token in response body
	}

	h.sendJSONResponse(w, http.StatusOK, response)
}

