package auth_handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"print-pro-backend/internal/middleware/auth_middleware"
	"print-pro-backend/internal/models"
	"print-pro-backend/internal/models/account"

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

	// IMPORTANT: After password verification, we need to ensure the account type matches
	// This prevents someone from logging in with a customer account when they should be a partner, or vice versa
	// The user_type is determined from the database, not from the request
	// If someone registered as customer and tries to login, they'll login as customer (correct)
	// If someone registered as partner and tries to login, they'll login as partner (correct)
	// This validation happens after password check to ensure security

	// Trim any whitespace from the password hash stored in database
	passwordHashFromDB := strings.TrimSpace(accountRecord.PasswordHash)
	
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
	accessToken, _, err := h.generateTokensAndSetCookies(w, authUser.ID, authUser.Email, accountRecord.UserType, ctx)
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

// LoginPartner handles partner login with email and password
// Only allows accounts with user_type = "partner" to login
func (h *AuthHandler) LoginPartner(w http.ResponseWriter, r *http.Request) {
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

	log.Printf("Account found - ID: %d, UserType: %s", accountRecord.ID, accountRecord.UserType)

	// VALIDATION: Only allow partners to login through this endpoint
	if accountRecord.UserType != account.UserTypePartner {
		log.Printf("ERROR: Attempted partner login with non-partner account - UserType: %s", accountRecord.UserType)
		h.sendErrorResponse(w, http.StatusForbidden, "Invalid account type", "This email is registered as a customer. Please use the customer login.")
		return
	}

	// Check if account is OAuth user (can't login with password)
	if accountRecord.PasswordHash == "oauth_google" {
		log.Printf("ERROR: Attempted password login for OAuth user - UserType: %s", accountRecord.UserType)
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

	log.Printf("SUCCESS: Partner password verified - authentication successful")

	// Convert account to auth user model
	authUser := &models.User{
		ID:        fmt.Sprintf("%d", accountRecord.ID),
		Email:     accountRecord.Email,
		Name:      accountRecord.Email, // Will be updated based on profile if needed
		UserType:  accountRecord.UserType, // Set user_type from account (should be "partner")
		Provider:  "email",
		CreatedAt: accountRecord.CreatedAt,
		UpdatedAt: accountRecord.CreatedAt,
	}

	// Generate tokens and set cookies (include user_type in JWT)
	accessToken, _, err := h.generateTokensAndSetCookies(w, authUser.ID, authUser.Email, accountRecord.UserType, ctx)
	if err != nil {
		return // Error already handled in generateTokensAndSetCookies
	}

	// Return success response with access token (refresh token in cookie)
	response := models.GoogleSignInResponse{
		Success: true,
		Message: "Partner login successful",
		User:    authUser,
		Token:   accessToken, // Access token in response body
	}

	h.sendJSONResponse(w, http.StatusOK, response)
}

// LoginCustomer handles customer login with email and password
// Only allows accounts with user_type = "customer" to login
func (h *AuthHandler) LoginCustomer(w http.ResponseWriter, r *http.Request) {
	log.Printf("LoginCustomer endpoint hit - Method: %s, URL: %s", r.Method, r.URL.Path)

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
	log.Printf("Fetching account from database for customer login")
	accountRecord, err := h.accountRepository.GetByEmail(ctx, email)
	if err != nil {
		log.Printf("ERROR: Account not found - Error: %v", err)
		h.sendErrorResponse(w, http.StatusUnauthorized, "Invalid credentials", "Invalid email or password")
		return
	}

	log.Printf("Account found - ID: %d, UserType: %s", accountRecord.ID, accountRecord.UserType)

	// VALIDATION: Only allow customers to login through this endpoint
	if accountRecord.UserType != account.UserTypeCustomer {
		log.Printf("ERROR: Attempted customer login with non-customer account - UserType: %s", accountRecord.UserType)
		h.sendErrorResponse(w, http.StatusForbidden, "Invalid account type", "This email is registered as a partner. Please use the partner login.")
		return
	}

	// Check if account is OAuth user (can't login with password)
	if accountRecord.PasswordHash == "oauth_google" {
		log.Printf("ERROR: Attempted password login for OAuth user - UserType: %s", accountRecord.UserType)
		h.sendErrorResponse(w, http.StatusBadRequest, "Invalid login method", "This account uses Google Sign-In. Please use Google to sign in.")
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

	log.Printf("SUCCESS: Customer password verified - authentication successful")

	// Convert account to auth user model
	authUser := &models.User{
		ID:        fmt.Sprintf("%d", accountRecord.ID),
		Email:     accountRecord.Email,
		Name:      accountRecord.Email, // Will be updated based on profile if needed
		UserType:  accountRecord.UserType, // Set user_type from account (should be "customer")
		Provider:  "email",
		CreatedAt: accountRecord.CreatedAt,
		UpdatedAt: accountRecord.CreatedAt,
	}

	// Generate tokens and set cookies (include user_type in JWT)
	accessToken, _, err := h.generateTokensAndSetCookies(w, authUser.ID, authUser.Email, accountRecord.UserType, ctx)
	if err != nil {
		return // Error already handled in generateTokensAndSetCookies
	}

	// Return success response with access token (refresh token in cookie)
	response := models.GoogleSignInResponse{
		Success: true,
		Message: "Customer login successful",
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

	// Generate tokens and set cookies (old Register handler - defaults to customer)
	// TODO: This handler is deprecated, use RegisterPartner or RegisterCustomer instead
	accessToken, _, err := h.generateTokensAndSetCookies(w, registeredUser.ID, registeredUser.Email, "customer", ctx)
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

// GoogleSignIn handles Google Sign-In requests (legacy unified endpoint - deprecated)
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
	log.Printf("🔍 GOOGLE: Verifying Google ID token...")
	user, err := h.googleAuthService.VerifyGoogleToken(r.Context(), req.Token)
	if err != nil {
		log.Printf("❌ GOOGLE: Token verification failed - %v", err)
		h.sendErrorResponse(w, http.StatusUnauthorized, "Token verification failed", err.Error())
		return
	}
	log.Printf("✅ GOOGLE: Token verified successfully")

	// Register or login user (backend automatically detects user_type from accounts table)
	log.Printf("🔍 GOOGLE: Registering or logging in user (auto-detecting user_type)...")
	registeredUser, err := h.googleAuthService.RegisterOrLoginUser(r.Context(), user)
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
	// Get user_type from registeredUser (set by RegisterOrLoginUser based on account)
	// Use the user_type from the registered user (which was set based on request or existing account)
	finalUserType := registeredUser.UserType
	if finalUserType == "" {
		// Fallback to customer if user_type not set (for backward compatibility)
		finalUserType = account.UserTypeCustomer
	}
	accessToken, _, err := h.generateTokensAndSetCookies(w, registeredUser.ID, registeredUser.Email, finalUserType, r.Context())
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

// GoogleSignInPartner handles Google Sign-In for partners only
// Only allows accounts with user_type = "partner" to login via Google
func (h *AuthHandler) GoogleSignInPartner(w http.ResponseWriter, r *http.Request) {
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
	user, err := h.googleAuthService.VerifyGoogleToken(r.Context(), req.Token)
	if err != nil {
		log.Printf("❌ GOOGLE PARTNER: Token verification failed - %v", err)
		h.sendErrorResponse(w, http.StatusUnauthorized, "Token verification failed", err.Error())
		return
	}
	log.Printf("✅ GOOGLE PARTNER: Token verified successfully")

	// Register or login user (backend automatically detects user_type from accounts table)
	log.Printf("🔍 GOOGLE PARTNER: Registering or logging in user (auto-detecting user_type)...")
	registeredUser, err := h.googleAuthService.RegisterOrLoginUser(r.Context(), user)
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
	accessToken, _, err := h.generateTokensAndSetCookies(w, registeredUser.ID, registeredUser.Email, registeredUser.UserType, r.Context())
	if err != nil {
		return // Error already handled in generateTokensAndSetCookies
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

// GoogleSignInCustomer handles Google Sign-In for customers only
// Only allows accounts with user_type = "customer" to login via Google
func (h *AuthHandler) GoogleSignInCustomer(w http.ResponseWriter, r *http.Request) {
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
	log.Printf("🔍 GOOGLE CUSTOMER: Verifying Google ID token...")
	user, err := h.googleAuthService.VerifyGoogleToken(r.Context(), req.Token)
	if err != nil {
		log.Printf("❌ GOOGLE CUSTOMER: Token verification failed - %v", err)
		h.sendErrorResponse(w, http.StatusUnauthorized, "Token verification failed", err.Error())
		return
	}
	log.Printf("✅ GOOGLE CUSTOMER: Token verified successfully")

	// Register or login user (backend automatically detects user_type from accounts table)
	log.Printf("🔍 GOOGLE CUSTOMER: Registering or logging in user (auto-detecting user_type)...")
	registeredUser, err := h.googleAuthService.RegisterOrLoginUser(r.Context(), user)
	if err != nil {
		log.Printf("❌ GOOGLE CUSTOMER: Failed to register/login user - %v", err)
		// Check if error is about account not found (Closed Loop enrollment)
		if strings.Contains(err.Error(), "account not found") || strings.Contains(err.Error(), "register first") {
			h.sendErrorResponse(w, http.StatusNotFound, "Account not found", "Account not found. Please register as a customer first using email/password registration.")
			return
		}
		h.sendErrorResponse(w, http.StatusInternalServerError, "Failed to register/login user", err.Error())
		return
	}
	log.Printf("✅ GOOGLE CUSTOMER: User registered/logged in - ID: %s, UserType: %s", registeredUser.ID, registeredUser.UserType)

	// VALIDATION: Only allow customers to login through this endpoint
	if registeredUser.UserType != account.UserTypeCustomer {
		log.Printf("ERROR: Attempted customer Google Sign-In with non-customer account - UserType: %s", registeredUser.UserType)
		h.sendErrorResponse(w, http.StatusForbidden, "Invalid account type", "This email is registered as a partner. Please use the partner Google Sign-In.")
		return
	}

	// Generate tokens and set cookies
	accessToken, _, err := h.generateTokensAndSetCookies(w, registeredUser.ID, registeredUser.Email, registeredUser.UserType, r.Context())
	if err != nil {
		return // Error already handled in generateTokensAndSetCookies
	}

	// Send success response with access token (refresh token in cookie)
	response := models.GoogleSignInResponse{
		Success: true,
		Message: "Customer authenticated successfully via Google",
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

	// Generate new access token (use user_type from claims)
	log.Printf("🔑 REFRESH: Generating new access token (expires in 15 minutes)")
	newAccessToken, err := h.jwtService.GenerateAccessToken(claims.UserID, claims.Email, claims.UserType)
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
	user, ok := auth_middleware.GetUserFromContext(r)
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

// CheckUserType returns the user type of the authenticated user
func (h *AuthHandler) CheckUserType(w http.ResponseWriter, r *http.Request) {
	// Only allow GET requests
	if r.Method != http.MethodGet {
		h.sendErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed", "Only GET method is allowed")
		return
	}

	// Get user from context (set by auth middleware)
	user, ok := auth_middleware.GetUserFromContext(r)
	if !ok {
		h.sendErrorResponse(w, http.StatusUnauthorized, "Unauthorized", "User not found in session")
		return
	}

	// Get user_type from user object (set by auth middleware)
	// The auth middleware sets user.UserType from JWT claims
	if user.UserType == "" {
		h.sendErrorResponse(w, http.StatusInternalServerError, "Internal error", "User type not found")
		return
	}

	// Return user type in the format expected by frontend
	response := map[string]interface{}{
		"success":   true,
		"user_type": user.UserType,
	}

	h.sendJSONResponse(w, http.StatusOK, response)
}

// generateTokensAndSetCookies generates JWT tokens and sets refresh token cookie
// Returns accessToken and refreshToken, or error if generation fails
func (h *AuthHandler) generateTokensAndSetCookies(w http.ResponseWriter, userID, email, userType string, ctx context.Context) (string, string, error) {
	// Generate JWT access token (15 minutes expiry)
	log.Printf("🔑 Generating access token (expires in 15 minutes) for %s: %s", userType, userID)
	accessToken, err := h.jwtService.GenerateAccessToken(userID, email, userType)
	if err != nil {
		log.Printf("ERROR: Failed to generate access token - %v", err)
		h.sendErrorResponse(w, http.StatusInternalServerError, "Failed to generate access token", err.Error())
		return "", "", err
	}
	log.Printf("✅ Access token generated successfully for user: %s", userID)

	// Generate JWT refresh token (7 days expiry)
	log.Printf("🔑 Generating refresh token (expires in 7 days)")
	refreshToken, err := h.jwtService.GenerateRefreshToken(userID, email, userType)
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

