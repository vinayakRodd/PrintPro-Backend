package customer

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

// RegisterHandler handles customer registration
type RegisterHandler struct {
	accountRepository         *repositories.AccountRepository
	customerProfileRepository *repositories.CustomerProfileRepository
	tokenHelper               shared.TokenHelper
	sendErrorResponse         func(http.ResponseWriter, int, string, string)
	sendJSONResponse          func(http.ResponseWriter, int, interface{})
}

// NewRegisterHandler creates a new RegisterHandler instance
func NewRegisterHandler(
	accountRepository *repositories.AccountRepository,
	customerProfileRepository *repositories.CustomerProfileRepository,
	tokenHelper shared.TokenHelper,
	sendErrorResponse func(http.ResponseWriter, int, string, string),
	sendJSONResponse func(http.ResponseWriter, int, interface{}),
) *RegisterHandler {
	return &RegisterHandler{
		accountRepository:         accountRepository,
		customerProfileRepository: customerProfileRepository,
		tokenHelper:               tokenHelper,
		sendErrorResponse:         sendErrorResponse,
		sendJSONResponse:          sendJSONResponse,
	}
}

// HandleRegister handles customer registration
func (h *RegisterHandler) HandleRegister(w http.ResponseWriter, r *http.Request) {
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
	// Extract username from request (optional field)
	var usernamePtr *string
	if req.Username != "" {
		username := strings.TrimSpace(req.Username)
		usernamePtr = &username
	}
	
	accountRecord, err := h.accountRepository.Create(ctx, email, string(hashedPassword), account.UserTypeCustomer, usernamePtr)
	if err != nil {
		log.Printf("ERROR: Failed to create account in accounts table - %v", err)
		log.Printf("ERROR: UserType: %s", account.UserTypeCustomer)
		h.sendErrorResponse(w, http.StatusInternalServerError, "Failed to create account", err.Error())
		return
	}

	// Step 2: Create customer profile
	_, err = h.customerProfileRepository.Create(ctx, accountRecord.Email, "")
	if err != nil {
		log.Printf("ERROR: Failed to create customer profile - %v", err)
		h.sendErrorResponse(w, http.StatusInternalServerError, "Failed to create customer profile", err.Error())
		return
	}

	log.Printf("SUCCESS: Customer registered successfully")

	// Convert account to auth user model
	registeredUser := &models.User{
		ID:        accountRecord.Email, // Use email as ID
		Email:     accountRecord.Email,
		Name:      accountRecord.Email, // Use email as display name for customers
		UserType:  accountRecord.UserType, // Set user_type from account (should be "customer")
		Provider:  "email",
		CreatedAt: accountRecord.CreatedAt,
		UpdatedAt: accountRecord.CreatedAt,
	}

	// Generate tokens and set cookies
	accessToken, _, err := h.tokenHelper.GenerateTokensAndSetCookies(w, registeredUser.ID, registeredUser.Email, accountRecord.UserType, ctx)
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

