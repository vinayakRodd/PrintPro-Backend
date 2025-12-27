package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
	"print-pro-backend/internal/config"
	"print-pro-backend/internal/infrastructure"
	"print-pro-backend/internal/middleware"
	"print-pro-backend/internal/models"
	userModel "print-pro-backend/internal/models/user"
	"print-pro-backend/internal/repositories"
	"print-pro-backend/internal/services"

	"golang.org/x/crypto/bcrypt"
)

// AuthHandler handles authentication-related requests
type AuthHandler struct {
	googleAuthService *services.GoogleAuthService
	sessionService    *services.SessionService
	emailService      *services.EmailService
	otpService        *services.OTPService
	userRepository    *repositories.UserRepository
	redisClient       *infrastructure.RedisClient
	jwtService        *services.JWTService
	config            *config.Config
}

// NewAuthHandler creates a new AuthHandler instance
func NewAuthHandler(
	googleAuthService *services.GoogleAuthService,
	sessionService *services.SessionService,
	emailService *services.EmailService,
	otpService *services.OTPService,
	userRepository *repositories.UserRepository,
	redisClient *infrastructure.RedisClient,
	jwtService *services.JWTService,
	cfg *config.Config,
) *AuthHandler {
	return &AuthHandler{
		googleAuthService: googleAuthService,
		sessionService:    sessionService,
		emailService:      emailService,
		otpService:        otpService,
		userRepository:    userRepository,
		redisClient:       redisClient,
		jwtService:        jwtService,
		config:            cfg,
	}
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

	// Generate JWT access token (15 minutes expiry)
	log.Printf("🔑 GOOGLE_SIGNIN: Generating access token (expires in 15 minutes)")
	accessToken, err := h.jwtService.GenerateAccessToken(registeredUser.ID, registeredUser.Email)
	if err != nil {
		log.Printf("ERROR: Failed to generate access token - %v", err)
		h.sendErrorResponse(w, http.StatusInternalServerError, "Failed to generate access token", err.Error())
		return
	}
	log.Printf("✅ GOOGLE_SIGNIN: Access token generated successfully for user: %s", registeredUser.ID)

	// Generate JWT refresh token (7 days expiry)
	log.Printf("🔑 GOOGLE_SIGNIN: Generating refresh token (expires in 7 days)")
	refreshToken, err := h.jwtService.GenerateRefreshToken(registeredUser.ID, registeredUser.Email)
	if err != nil {
		log.Printf("ERROR: Failed to generate refresh token - %v", err)
		h.sendErrorResponse(w, http.StatusInternalServerError, "Failed to generate refresh token", err.Error())
		return
	}
	log.Printf("✅ GOOGLE_SIGNIN: Refresh token generated successfully for user: %s", registeredUser.ID)

	// Store refresh token in Redis (for revocation)
	ctx := r.Context()
	log.Printf("💾 GOOGLE_SIGNIN: Storing refresh token in Redis")
	if err := h.sessionService.StoreRefreshToken(ctx, refreshToken, registeredUser.ID); err != nil {
		log.Printf("ERROR: Failed to store refresh token in Redis: %v", err)
		h.sendErrorResponse(w, http.StatusInternalServerError, "Failed to store refresh token", err.Error())
		return
	}

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

// getUserFromCacheOrDB gets user from Redis cache first, then falls back to database
// Stores user in cache for future lookups (for scalability)
func (h *AuthHandler) getUserFromCacheOrDB(ctx context.Context, email string) (*userModel.User, error) {
	// Normalize email (lowercase, trim)
	email = strings.ToLower(strings.TrimSpace(email))
	
	// Check Redis cache first
	cacheKey := fmt.Sprintf("user:email:%s", email)
	cachedData, err := h.redisClient.Get(ctx, cacheKey)
	if err == nil && cachedData != "" {
		// User found in cache - parse JSON
		var dbUser userModel.User
		if err := json.Unmarshal([]byte(cachedData), &dbUser); err == nil {
			log.Printf("User found in cache")
			return &dbUser, nil
		}
	}
	
	// Not in cache - check database
	log.Printf("User not in cache, checking database")
	dbUser, err := h.userRepository.GetByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	
	// Store in cache for future lookups (1 hour TTL)
	userData, err := json.Marshal(dbUser)
	if err == nil {
		h.redisClient.Set(ctx, cacheKey, userData, time.Hour)
		log.Printf("User stored in cache")
	}
	
	return dbUser, nil
}

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

	// Generate JWT access token (15 minutes expiry)
	log.Printf("🔑 LOGIN: Generating access token (expires in 15 minutes)")
	accessToken, err := h.jwtService.GenerateAccessToken(authUser.ID, authUser.Email)
	if err != nil {
		log.Printf("ERROR: Failed to generate access token - %v", err)
		h.sendErrorResponse(w, http.StatusInternalServerError, "Failed to generate access token", err.Error())
		return
	}
	log.Printf("✅ LOGIN: Access token generated successfully for user: %s", authUser.ID)

	// Generate JWT refresh token (7 days expiry)
	log.Printf("🔑 LOGIN: Generating refresh token (expires in 7 days)")
	refreshToken, err := h.jwtService.GenerateRefreshToken(authUser.ID, authUser.Email)
	if err != nil {
		log.Printf("ERROR: Failed to generate refresh token - %v", err)
		h.sendErrorResponse(w, http.StatusInternalServerError, "Failed to generate refresh token", err.Error())
		return
	}
	log.Printf("✅ LOGIN: Refresh token generated successfully for user: %s", authUser.ID)

	// Store refresh token in Redis (for revocation)
	log.Printf("💾 LOGIN: Storing refresh token in Redis")
	if err := h.sessionService.StoreRefreshToken(ctx, refreshToken, authUser.ID); err != nil {
		log.Printf("ERROR: Failed to store refresh token in Redis: %v", err)
		h.sendErrorResponse(w, http.StatusInternalServerError, "Failed to store refresh token", err.Error())
		return
	}
	log.Printf("✅ LOGIN: Refresh token stored in Redis successfully")

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

	// Generate JWT access token (15 minutes expiry)
	log.Printf("🔑 REGISTER: Generating access token (expires in 15 minutes)")
	accessToken, err := h.jwtService.GenerateAccessToken(registeredUser.ID, registeredUser.Email)
	if err != nil {
		log.Printf("ERROR: Failed to generate access token - %v", err)
		h.sendErrorResponse(w, http.StatusInternalServerError, "Failed to generate access token", err.Error())
		return
	}
	log.Printf("✅ REGISTER: Access token generated successfully for user: %s", registeredUser.ID)

	// Generate JWT refresh token (7 days expiry)
	log.Printf("🔑 REGISTER: Generating refresh token (expires in 7 days)")
	refreshToken, err := h.jwtService.GenerateRefreshToken(registeredUser.ID, registeredUser.Email)
	if err != nil {
		log.Printf("ERROR: Failed to generate refresh token - %v", err)
		h.sendErrorResponse(w, http.StatusInternalServerError, "Failed to generate refresh token", err.Error())
		return
	}
	log.Printf("✅ REGISTER: Refresh token generated successfully for user: %s", registeredUser.ID)

	// Store refresh token in Redis (for revocation)
	log.Printf("💾 REGISTER: Storing refresh token in Redis")
	if err := h.sessionService.StoreRefreshToken(ctx, refreshToken, registeredUser.ID); err != nil {
		log.Printf("ERROR: Failed to store refresh token in Redis: %v", err)
		h.sendErrorResponse(w, http.StatusInternalServerError, "Failed to store refresh token", err.Error())
		return
	}

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

	// Return success response with access token (refresh token in cookie)
	response := models.GoogleSignInResponse{
		Success: true,
		Message: "User registered successfully",
		User:    registeredUser,
		Token:   accessToken, // Access token in response body
	}

	h.sendJSONResponse(w, http.StatusCreated, response)
}

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

// VerifyOTP handles OTP verification only (without password reset)
func (h *AuthHandler) VerifyOTP(w http.ResponseWriter, r *http.Request) {
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
	otp := strings.TrimSpace(req.OTP)
	
	log.Printf("Verifying OTP - OTP length: %d", len(otp))
	
	valid, err := h.otpService.VerifyOTP(ctx, email, otp)
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

// sendJSONResponse sends a JSON response
func (h *AuthHandler) sendJSONResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	
	// Encode JSON first to check for errors before writing header
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	if err := encoder.Encode(data); err != nil {
		log.Printf("ERROR: Failed to encode JSON response - Status: %d, Error: %v", statusCode, err)
		// If encoding fails, send error response
		w.WriteHeader(http.StatusInternalServerError)
		errorResponse := models.ErrorResponse{
			Success: false,
			Message: "Failed to encode response",
			Error:   err.Error(),
		}
		json.NewEncoder(w).Encode(errorResponse)
		return
	}
	
	// If encoding succeeded, write header and send response
	w.WriteHeader(statusCode)
	if _, err := w.Write(buf.Bytes()); err != nil {
		log.Printf("ERROR: Failed to write JSON response - Status: %d, Error: %v", statusCode, err)
	} else {
		log.Printf("JSON response sent successfully - Status: %d", statusCode)
	}
}

// sendErrorResponse sends an error response
func (h *AuthHandler) sendErrorResponse(w http.ResponseWriter, statusCode int, message, errorDetail string) {
	response := models.ErrorResponse{
		Success: false,
		Message: message,
		Error:   errorDetail,
	}
	h.sendJSONResponse(w, statusCode, response)
}

