package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"print-pro-backend/internal/middleware"
	"print-pro-backend/internal/models"

	"golang.org/x/crypto/bcrypt"
)

// Login handles user login with email and password
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	log.Printf("Login endpoint hit - Method: %s, URL: %s", r.Method, r.URL.Path)

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

	// For login, we need password hash, so always fetch from database (not cache)
	// The cache doesn't store password hash due to json:"-" tag for security
	log.Printf("Fetching user from database for login (password verification required)")
	dbUser, err := h.userRepository.GetByEmail(ctx, email)
	if err != nil {
		log.Printf("ERROR: User not found for email")
		h.sendErrorResponse(w, http.StatusUnauthorized, "Invalid credentials", "Invalid email or password")
		return
	}

	log.Printf("User found - ID: %d", dbUser.ID)

	// Check if user is OAuth user (can't login with password)
	if dbUser.PasswordHash == "oauth_google" {
		log.Printf("ERROR: Attempted password login for OAuth user")
		h.sendErrorResponse(w, http.StatusBadRequest, "Invalid login method", "This account uses Google Sign-In. Please use Google to sign in.")
		return
	}

	// Trim any whitespace from the password hash stored in database
	passwordHashFromDB := strings.TrimSpace(dbUser.PasswordHash)
	
	// Validate password hash format (bcrypt hashes start with $2a$, $2b$, or $2y$)
	if len(passwordHashFromDB) < 10 || !strings.HasPrefix(passwordHashFromDB, "$2") {
		log.Printf("ERROR: Invalid password hash format in database")
		h.sendErrorResponse(w, http.StatusInternalServerError, "Invalid password format", "Password format is invalid. Please reset your password.")
		return
	}
	
	// Verify password using bcrypt.CompareHashAndPassword
	// IMPORTANT: This function internally:
	// 1. Extracts the salt from the stored hash (passwordHashFromDB)
	// 2. Hashes the provided plain text password (password) with that salt
	// 3. Compares the newly hashed password with the stored hash
	// This is the correct way to verify bcrypt passwords - we don't need to hash manually
	if err := bcrypt.CompareHashAndPassword([]byte(passwordHashFromDB), []byte(password)); err != nil {
		log.Printf("ERROR: Password comparison failed")
		h.sendErrorResponse(w, http.StatusUnauthorized, "Invalid credentials", "Invalid email or password")
		return
	}

	log.Printf("SUCCESS: Password verified - authentication successful")

	// Convert database user to auth user model
	authUser := &models.User{
		ID:        fmt.Sprintf("%d", dbUser.ID),
		Email:     dbUser.Email,
		Name:      dbUser.FullName,
		Provider:  "email",
		CreatedAt: dbUser.CreatedAt,
		UpdatedAt: dbUser.CreatedAt,
	}

	// Generate tokens and set cookies
	accessToken, _, err := h.generateTokensAndSetCookies(w, authUser.ID, authUser.Email, ctx)
	if err != nil {
		return // Error already handled in generateTokensAndSetCookies
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

// Register handles user registration
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	log.Printf("Register endpoint hit - Method: %s, URL: %s", r.Method, r.URL.Path)

	if r.Method != http.MethodPost {
		h.sendErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed", "Only POST method is allowed")
		return
	}

	var req models.RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendErrorResponse(w, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

	// Validate input
	if req.FullName == "" {
		h.sendErrorResponse(w, http.StatusBadRequest, "Full name is required", "Full name field cannot be empty")
		return
	}

	if req.Email == "" {
		h.sendErrorResponse(w, http.StatusBadRequest, "Email is required", "Email field cannot be empty")
		return
	}

	if req.Password == "" {
		h.sendErrorResponse(w, http.StatusBadRequest, "Password is required", "Password field cannot be empty")
		return
	}

	// Normalize email
	email := strings.ToLower(strings.TrimSpace(req.Email))
	
	// Trim password (but don't lowercase it - passwords are case-sensitive)
	password := strings.TrimSpace(req.Password)
	
	if len(password) < 6 {
		h.sendErrorResponse(w, http.StatusBadRequest, "Password too short", "Password must be at least 6 characters")
		return
	}

	ctx := r.Context()

	// Check if user already exists
	existingUser, err := h.userRepository.GetByEmail(ctx, email)
	if err == nil && existingUser != nil {
		h.sendErrorResponse(w, http.StatusConflict, "User already exists", "A user with this email already exists")
		return
	}

	// Hash the password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("ERROR: Failed to hash password - %v", err)
		h.sendErrorResponse(w, http.StatusInternalServerError, "Failed to process password", err.Error())
		return
	}

	// Create user in database
	dbUser, err := h.userRepository.Create(ctx, strings.TrimSpace(req.FullName), email, string(hashedPassword))
	if err != nil {
		log.Printf("ERROR: Failed to create user - %v", err)
		h.sendErrorResponse(w, http.StatusInternalServerError, "Failed to create user", err.Error())
		return
	}

	log.Printf("SUCCESS: User registered - ID: %d", dbUser.ID)

	// Convert database user to auth user model
	registeredUser := &models.User{
		ID:        fmt.Sprintf("%d", dbUser.ID),
		Email:     dbUser.Email,
		Name:      dbUser.FullName,
		Provider:  "email", // Regular email/password registration
		CreatedAt: dbUser.CreatedAt,
		UpdatedAt: dbUser.CreatedAt,
	}

	// Generate tokens and set cookies
	accessToken, _, err := h.generateTokensAndSetCookies(w, registeredUser.ID, registeredUser.Email, ctx)
	if err != nil {
		return // Error already handled in generateTokensAndSetCookies
	}

	// Return success response with access token (refresh token in cookie)
	response := models.GoogleSignInResponse{
		Success: true,
		Message: "User registered successfully",
		User:    registeredUser,
		Token:   accessToken, // Access token in response body
	}

	h.sendJSONResponse(w, http.StatusCreated, response)
}

// GoogleSignIn handles Google Sign-In requests
// CORS is handled by corsHandler middleware in main.go
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
		h.sendErrorResponse(w, http.StatusUnauthorized, "Token verification failed", err.Error())
		return
	}

	// Register or login user
	registeredUser, err := h.googleAuthService.RegisterOrLoginUser(r.Context(), user)
	if err != nil {
		h.sendErrorResponse(w, http.StatusInternalServerError, "Failed to register/login user", err.Error())
		return
	}

	// Generate tokens and set cookies
	accessToken, _, err := h.generateTokensAndSetCookies(w, registeredUser.ID, registeredUser.Email, r.Context())
	if err != nil {
		return // Error already handled in generateTokensAndSetCookies
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

// RefreshToken handles refresh token requests to get a new access token
func (h *AuthHandler) RefreshToken(w http.ResponseWriter, r *http.Request) {
	log.Printf("🔄 REFRESH: Refresh token endpoint hit - Method: %s, URL: %s", r.Method, r.URL.Path)

	if r.Method != http.MethodPost {
		h.sendErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed", "Only POST method is allowed")
		return
	}

	// Get refresh token from cookie
	cookie, err := r.Cookie("refresh_token")
	if err != nil || cookie.Value == "" {
		log.Printf("❌ REFRESH: Refresh token cookie not found")
		h.sendErrorResponse(w, http.StatusUnauthorized, "Unauthorized", "Refresh token not found")
		return
	}

	refreshToken := cookie.Value
	ctx := r.Context()
	log.Printf("🔍 REFRESH: Refresh token found in cookie, validating...")

	// Validate refresh token JWT
	claims, err := h.jwtService.ValidateRefreshToken(refreshToken)
	if err != nil {
		log.Printf("❌ REFRESH: Invalid refresh token - %v", err)
		h.sendErrorResponse(w, http.StatusUnauthorized, "Unauthorized", "Invalid or expired refresh token")
		return
	}
	log.Printf("✅ REFRESH: Refresh token JWT validated successfully for user: %s", claims.UserID)

	// Verify refresh token exists in Redis (for revocation check)
	log.Printf("🔍 REFRESH: Checking refresh token in Redis...")
	_, err = h.sessionService.ValidateRefreshToken(ctx, refreshToken)
	if err != nil {
		log.Printf("❌ REFRESH: Refresh token not found in Redis (revoked) - %v", err)
		h.sendErrorResponse(w, http.StatusUnauthorized, "Unauthorized", "Refresh token has been revoked")
		return
	}
	log.Printf("✅ REFRESH: Refresh token found in Redis (not revoked)")

	// Generate new access token
	log.Printf("🔑 REFRESH: Generating new access token (expires in 15 minutes)")
	newAccessToken, err := h.jwtService.GenerateAccessToken(claims.UserID, claims.Email)
	if err != nil {
		log.Printf("ERROR: Failed to generate new access token - %v", err)
		h.sendErrorResponse(w, http.StatusInternalServerError, "Failed to generate access token", err.Error())
		return
	}

	log.Printf("✅ REFRESH: New access token generated successfully for user: %s", claims.UserID)
	log.Printf("🎉 REFRESH: Token refresh completed - user can continue without re-login")

	// Return new access token
	h.sendJSONResponse(w, http.StatusOK, map[string]interface{}{
		"success":      true,
		"access_token": newAccessToken,
		"message":      "Access token refreshed successfully",
	})
}

// Logout handles user logout by deleting the refresh token
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	// Only allow POST requests
	if r.Method != http.MethodPost {
		h.sendErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed", "Only POST method is allowed")
		return
	}

	ctx := r.Context()

	// Get refresh token from cookie and delete it from Redis
	refreshCookie, err := r.Cookie("refresh_token")
	if err == nil && refreshCookie.Value != "" {
		// Delete refresh token from Redis
		if err := h.sessionService.DeleteRefreshToken(ctx, refreshCookie.Value); err != nil {
			log.Printf("Failed to delete refresh token from Redis: %v", err)
		}
	}

	// Delete the refresh token cookie by setting it with MaxAge: -1
	deleteCookie := &http.Cookie{
		Name:     "refresh_token",
		Value:    "",
		Path:     "/",
		MaxAge:   -1, // Delete the cookie
		HttpOnly: true,
		Secure:   h.config.SecureCookies,
		SameSite: http.SameSiteLaxMode,
	}
	
	// Set domain if configured
	if h.config.CookieDomain != "" {
		deleteCookie.Domain = h.config.CookieDomain
	}
	
	http.SetCookie(w, deleteCookie)
	log.Printf("Refresh token cookie deleted from browser")

	// Send success response
	response := models.GoogleSignInResponse{
		Success: true,
		Message: "Logged out successfully",
		User:    nil,
		Token:   "",
	}

	h.sendJSONResponse(w, http.StatusOK, response)
}

// GetMe returns the current authenticated user
func (h *AuthHandler) GetMe(w http.ResponseWriter, r *http.Request) {
	// Get user from context (set by auth middleware)
	user, ok := middleware.GetUserFromContext(r)
	if !ok {
		h.sendErrorResponse(w, http.StatusUnauthorized, "Unauthorized", "User not found in session")
		return
	}

	// Return user information
	response := models.GoogleSignInResponse{
		Success: true,
		Message: "User authenticated",
		User:    user,
		Token:   "", // Don't return token in /me endpoint
	}

	h.sendJSONResponse(w, http.StatusOK, response)
}

// generateTokensAndSetCookies generates JWT tokens and sets refresh token cookie
// Returns accessToken and refreshToken, or error if generation fails
func (h *AuthHandler) generateTokensAndSetCookies(w http.ResponseWriter, userID, email string, ctx context.Context) (string, string, error) {
	// Generate JWT access token (15 minutes expiry)
	log.Printf("🔑 Generating access token (expires in 15 minutes)")
	accessToken, err := h.jwtService.GenerateAccessToken(userID, email)
	if err != nil {
		log.Printf("ERROR: Failed to generate access token - %v", err)
		h.sendErrorResponse(w, http.StatusInternalServerError, "Failed to generate access token", err.Error())
		return "", "", err
	}
	log.Printf("✅ Access token generated successfully for user: %s", userID)

	// Generate JWT refresh token (7 days expiry)
	log.Printf("🔑 Generating refresh token (expires in 7 days)")
	refreshToken, err := h.jwtService.GenerateRefreshToken(userID, email)
	if err != nil {
		log.Printf("ERROR: Failed to generate refresh token - %v", err)
		h.sendErrorResponse(w, http.StatusInternalServerError, "Failed to generate refresh token", err.Error())
		return "", "", err
	}
	log.Printf("✅ Refresh token generated successfully for user: %s", userID)

	// Store refresh token in Redis (for revocation)
	log.Printf("💾 Storing refresh token in Redis")
	if err := h.sessionService.StoreRefreshToken(ctx, refreshToken, userID); err != nil {
		log.Printf("ERROR: Failed to store refresh token in Redis: %v", err)
		h.sendErrorResponse(w, http.StatusInternalServerError, "Failed to store refresh token", err.Error())
		return "", "", err
	}
	log.Printf("✅ Refresh token stored in Redis successfully")

	// Set secure HTTP-only cookie for refresh token
	refreshCookie := &http.Cookie{
		Name:     "refresh_token",
		Value:    refreshToken,
		Path:     "/",
		MaxAge:   7 * 24 * 60 * 60, // 7 days
		HttpOnly: true,              // Prevents JavaScript access (XSS protection)
		Secure:   h.config.SecureCookies, // Set via config (true in production with HTTPS)
		SameSite: http.SameSiteLaxMode,   // CSRF protection
	}
	
	// Set domain if configured
	if h.config.CookieDomain != "" {
		refreshCookie.Domain = h.config.CookieDomain
	}
	
	http.SetCookie(w, refreshCookie)
	log.Printf("Refresh token cookie set in browser")

	return accessToken, refreshToken, nil
}

