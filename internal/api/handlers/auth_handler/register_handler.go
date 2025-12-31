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

func (h *AuthHandler) RegisterPartner(w http.ResponseWriter, r *http.Request) {
	log.Printf("RegisterPartner endpoint hit - Method: %s, URL: %s", r.Method, r.URL.Path)

	if r.Method != http.MethodPost {
		h.sendErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed", "Only POST method is allowed")
		return
	}

	var req models.RegisterPartnerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendErrorResponse(w, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

	// Validate input
	if req.Email == "" {
		h.sendErrorResponse(w, http.StatusBadRequest, "Email is required", "Email field cannot be empty")
		return
	}

	if req.Password == "" {
		h.sendErrorResponse(w, http.StatusBadRequest, "Password is required", "Password field cannot be empty")
		return
	}

	if req.ShopName == "" {
		h.sendErrorResponse(w, http.StatusBadRequest, "Shop name is required", "shop_name field cannot be empty")
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

	// Check if account already exists
	existingAccount, err := h.accountRepository.GetByEmail(ctx, email)
	if err == nil && existingAccount != nil {
		// If account exists as customer, show specific message
		if existingAccount.UserType == account.UserTypeCustomer {
			h.sendErrorResponse(w, http.StatusConflict, "Email already registered as customer", "Please use a different email to create a partner account")
			return
		}
		// If account exists as partner, show generic message
		h.sendErrorResponse(w, http.StatusConflict, "Account already exists", "An account with this email already exists")
		return
	}

	// Hash the password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("ERROR: Failed to hash password - %v", err)
		h.sendErrorResponse(w, http.StatusInternalServerError, "Failed to process password", err.Error())
		return
	}

	// Step 1: Create account in accounts table
	accountRecord, err := h.accountRepository.Create(ctx, email, string(hashedPassword), account.UserTypePartner)
	if err != nil {
		log.Printf("ERROR: Failed to create account in accounts table - %v", err)
		log.Printf("ERROR: UserType: %s", account.UserTypePartner)
		h.sendErrorResponse(w, http.StatusInternalServerError, "Failed to create account", err.Error())
		return
	}

	// Step 2: Create partner profile
	_, err = h.partnerProfileRepository.Create(ctx, accountRecord.ID, strings.TrimSpace(req.ShopName), strings.TrimSpace(req.PrinterID))
	if err != nil {
		log.Printf("ERROR: Failed to create partner profile - %v", err)
		log.Printf("ERROR: AccountID: %d, ShopName: %s", accountRecord.ID, req.ShopName)
		h.sendErrorResponse(w, http.StatusInternalServerError, "Failed to create partner profile", err.Error())
		return
	}

	log.Printf("SUCCESS: Partner registered - Account ID: %d", accountRecord.ID)

	// Convert account to auth user model
	registeredUser := &models.User{
		ID:        fmt.Sprintf("%d", accountRecord.ID),
		Email:     accountRecord.Email,
		Name:      req.ShopName, // Use shop name as display name
		UserType:  accountRecord.UserType, // Set user_type from account (should be "partner")
		Provider:  "email",
		CreatedAt: accountRecord.CreatedAt,
		UpdatedAt: accountRecord.CreatedAt,
	}

	// Generate tokens and set cookies
	accessToken, _, err := h.generateTokensAndSetCookies(w, registeredUser.ID, registeredUser.Email, accountRecord.UserType, ctx)
	if err != nil {
		return // Error already handled in generateTokensAndSetCookies
	}

	// Return success response with access token (refresh token in cookie)
	response := models.GoogleSignInResponse{
		Success: true,
		Message: "Partner registered successfully",
		User:    registeredUser,
		Token:   accessToken, // Access token in response body
	}

	h.sendJSONResponse(w, http.StatusCreated, response)
}

// RegisterCustomer handles customer registration with transaction
func (h *AuthHandler) RegisterCustomer(w http.ResponseWriter, r *http.Request) {
	log.Printf("RegisterCustomer endpoint hit - Method: %s, URL: %s", r.Method, r.URL.Path)

	if r.Method != http.MethodPost {
		h.sendErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed", "Only POST method is allowed")
		return
	}

	var req models.RegisterCustomerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendErrorResponse(w, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

	// Validate input
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

	// Check if account already exists
	existingAccount, err := h.accountRepository.GetByEmail(ctx, email)
	if err == nil && existingAccount != nil {
		// If account exists as partner, show specific message
		if existingAccount.UserType == account.UserTypePartner {
			h.sendErrorResponse(w, http.StatusConflict, "Email already registered as partner", "Please use a different email to create a customer account")
			return
		}
		// If account exists as customer, show generic message
		h.sendErrorResponse(w, http.StatusConflict, "Account already exists", "An account with this email already exists")
		return
	}

	// Hash the password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("ERROR: Failed to hash password - %v", err)
		h.sendErrorResponse(w, http.StatusInternalServerError, "Failed to process password", err.Error())
		return
	}

	// Step 1: Create account in accounts table
	accountRecord, err := h.accountRepository.Create(ctx, email, string(hashedPassword), account.UserTypeCustomer)
	if err != nil {
		log.Printf("ERROR: Failed to create account in accounts table - %v", err)
		log.Printf("ERROR: UserType: %s", account.UserTypeCustomer)
		h.sendErrorResponse(w, http.StatusInternalServerError, "Failed to create account", err.Error())
		return
	}

	// Step 2: Create customer profile
	_, err = h.customerProfileRepository.Create(ctx, accountRecord.ID, "")
	if err != nil {
		log.Printf("ERROR: Failed to create customer profile - %v", err)
		h.sendErrorResponse(w, http.StatusInternalServerError, "Failed to create customer profile", err.Error())
		return
	}

	log.Printf("SUCCESS: Customer registered - Account ID: %d", accountRecord.ID)

	// Convert account to auth user model
	registeredUser := &models.User{
		ID:        fmt.Sprintf("%d", accountRecord.ID),
		Email:     accountRecord.Email,
		Name:      accountRecord.Email, // Use email as display name for customers
		UserType:  accountRecord.UserType, // Set user_type from account (should be "customer")
		Provider:  "email",
		CreatedAt: accountRecord.CreatedAt,
		UpdatedAt: accountRecord.CreatedAt,
	}

	// Generate tokens and set cookies
	accessToken, _, err := h.generateTokensAndSetCookies(w, registeredUser.ID, registeredUser.Email, accountRecord.UserType, ctx)
	if err != nil {
		return // Error already handled in generateTokensAndSetCookies
	}

	// Return success response with access token (refresh token in cookie)
	response := models.GoogleSignInResponse{
		Success: true,
		Message: "Customer registered successfully",
		User:    registeredUser,
		Token:   accessToken, // Access token in response body
	}

	h.sendJSONResponse(w, http.StatusCreated, response)
}


