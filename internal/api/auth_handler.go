package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"print-pro-backend/internal/config"
	"print-pro-backend/internal/middleware"
	"print-pro-backend/internal/models"
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
	config            *config.Config
}

// NewAuthHandler creates a new AuthHandler instance
func NewAuthHandler(
	googleAuthService *services.GoogleAuthService,
	sessionService *services.SessionService,
	emailService *services.EmailService,
	otpService *services.OTPService,
	userRepository *repositories.UserRepository,
	cfg *config.Config,
) *AuthHandler {
	return &AuthHandler{
		googleAuthService: googleAuthService,
		sessionService:    sessionService,
		emailService:      emailService,
		otpService:        otpService,
		userRepository:    userRepository,
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

	// Generate session token
	sessionToken, err := h.googleAuthService.GenerateSessionToken(registeredUser.ID)
	if err != nil {
		h.sendErrorResponse(w, http.StatusInternalServerError, "Failed to generate session token", err.Error())
		return
	}

	// Store session in Redis
	ctx := r.Context()
	if err := h.sessionService.CreateSession(ctx, sessionToken, registeredUser); err != nil {
		log.Printf("Failed to create session in Redis: %v", err)
		h.sendErrorResponse(w, http.StatusInternalServerError, "Failed to create session", err.Error())
		return
	}

	// Set secure HTTP-only cookie for the session token
	cookie := &http.Cookie{
		Name:     "session_token",
		Value:    sessionToken,
		Path:     "/",
		MaxAge:   7 * 24 * 60 * 60, // 7 days
		HttpOnly: true,              // Prevents JavaScript access (XSS protection)
		Secure:   h.config.SecureCookies, // Set via config (true in production with HTTPS)
		SameSite: http.SameSiteLaxMode,   // CSRF protection
	}
	
	// Set domain if configured
	if h.config.CookieDomain != "" {
		cookie.Domain = h.config.CookieDomain
	}
	
	http.SetCookie(w, cookie)
	log.Printf("Cookie set in browser")

	// Send success response (token also in response for frontend flexibility)
	response := models.GoogleSignInResponse{
		Success: true,
		Message: "User authenticated successfully",
		User:    registeredUser,
		Token:   sessionToken, // Still include in response if frontend needs it
	}

	h.sendJSONResponse(w, http.StatusOK, response)
}

// Logout handles user logout by deleting the session cookie
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	// Only allow POST requests
	if r.Method != http.MethodPost {
		h.sendErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed", "Only POST method is allowed")
		return
	}

	// Get session token from cookie
	cookie, err := r.Cookie("session_token")
	if err == nil && cookie.Value != "" {
		// Delete session from Redis
		ctx := r.Context()
		if err := h.sessionService.DeleteSession(ctx, cookie.Value); err != nil {
			log.Printf("Failed to delete session from Redis: %v", err)
		}
	}

	// Delete the session cookie by setting it with MaxAge: -1
	deleteCookie := &http.Cookie{
		Name:     "session_token",
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
	log.Printf("Cookie deleted from browser")

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
func (h *AuthHandler) GetEmail(w http.ResponseWriter, r *http.Request) {
	// Get user from context (set by auth middleware)
	user, ok := middleware.GetUserFromContext(r)
	if !ok {
		h.sendErrorResponse(w, http.StatusUnauthorized, "Unauthorized", "User not found in session")
		return
	}

	// Return email in response
	h.sendJSONResponse(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"email":   user.Email,
		"message": "Email retrieved successfully",
	})
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

	if len(req.Password) < 6 {
		h.sendErrorResponse(w, http.StatusBadRequest, "Password too short", "Password must be at least 6 characters")
		return
	}

	ctx := r.Context()

	// Check if user already exists
	existingUser, err := h.userRepository.GetByEmail(ctx, req.Email)
	if err == nil && existingUser != nil {
		h.sendErrorResponse(w, http.StatusConflict, "User already exists", "A user with this email already exists")
		return
	}

	// Hash the password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("ERROR: Failed to hash password - %v", err)
		h.sendErrorResponse(w, http.StatusInternalServerError, "Failed to process password", err.Error())
		return
	}

	// Create user in database
	dbUser, err := h.userRepository.Create(ctx, req.FullName, req.Email, string(hashedPassword))
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

	// Generate session token
	sessionToken, err := h.googleAuthService.GenerateSessionToken(registeredUser.ID)
	if err != nil {
		log.Printf("ERROR: Failed to generate session token - %v", err)
		h.sendErrorResponse(w, http.StatusInternalServerError, "Failed to generate session token", err.Error())
		return
	}

	// Store session in Redis
	if err := h.sessionService.CreateSession(ctx, sessionToken, registeredUser); err != nil {
		log.Printf("ERROR: Failed to create session in Redis: %v", err)
		h.sendErrorResponse(w, http.StatusInternalServerError, "Failed to create session", err.Error())
		return
	}

	// Set secure HTTP-only cookie for the session token
	cookie := &http.Cookie{
		Name:     "session_token",
		Value:    sessionToken,
		Path:     "/",
		MaxAge:   7 * 24 * 60 * 60, // 7 days
		HttpOnly: true,              // Prevents JavaScript access (XSS protection)
		Secure:   h.config.SecureCookies, // Set via config (true in production with HTTPS)
		SameSite: http.SameSiteLaxMode,   // CSRF protection
	}
	
	// Set domain if configured
	if h.config.CookieDomain != "" {
		cookie.Domain = h.config.CookieDomain
	}
	
	http.SetCookie(w, cookie)
	log.Printf("Cookie set in browser")

	// Return success response with user info
	response := models.GoogleSignInResponse{
		Success: true,
		Message: "User registered successfully",
		User:    registeredUser,
		Token:   sessionToken, // Include token in response for frontend flexibility
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
		h.sendErrorResponse(w, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

	// Validate email
	if req.Email == "" {
		h.sendErrorResponse(w, http.StatusBadRequest, "Email is required", "Email field cannot be empty")
		return
	}

	log.Printf("Forgot password API hit - generating OTP")

	// Always generate and send OTP regardless of whether user exists
	// This prevents email enumeration attacks
	ctx := r.Context()
	
	log.Printf("Generating OTP for email")
	// Generate OTP
	otp, err := h.otpService.GenerateOTP(ctx, req.Email)
	if err != nil {
		log.Printf("Failed to generate OTP: %v", err)
		h.sendErrorResponse(w, http.StatusInternalServerError, "Failed to generate OTP", err.Error())
		return
	}
	log.Printf("OTP generated successfully")

	log.Printf("Sending OTP to email")
	// Send OTP via email (always send, even if user doesn't exist)
	err = h.emailService.SendOTPEmail(req.Email, otp)
	if err != nil {
		log.Printf("Failed to send OTP email: %v", err)
		// Return error response with details
		h.sendErrorResponse(w, http.StatusInternalServerError, "Failed to send OTP email", err.Error())
		return
	}

	log.Printf("OTP sent successfully")

	// Return success response
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
	
	log.Printf("Verifying OTP for email, OTP length: %d", len(otp))
	
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
	password := req.NewPassword
	if password == "" {
		// Fallback to password field for backward compatibility
		password = req.Password
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

	// Check if reset token exists (OTP was already verified)
	valid, err := h.otpService.VerifyResetToken(ctx, req.Email)
	if err != nil || !valid {
		log.Printf("ERROR: Reset token not found or expired")
		h.sendErrorResponse(w, http.StatusUnauthorized, "OTP verification required", "Please verify your OTP first before resetting password")
		return
	}

	// Check if user exists
	user, err := h.userRepository.GetByEmail(ctx, req.Email)
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
	if err := h.userRepository.UpdatePassword(ctx, req.Email, string(hashedPassword)); err != nil {
		log.Printf("ERROR: Failed to update password in database - %v", err)
		h.sendErrorResponse(w, http.StatusInternalServerError, "Failed to update password", err.Error())
		return
	}

	// Delete reset token after successful password reset
	h.otpService.DeleteResetToken(ctx, req.Email)

	log.Printf("SUCCESS: Password reset completed")

	h.sendJSONResponse(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Password has been reset successfully",
	})
}

// sendJSONResponse sends a JSON response
func (h *AuthHandler) sendJSONResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("Failed to encode JSON response: %v", err)
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

