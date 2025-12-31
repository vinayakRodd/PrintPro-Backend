package customer

import (
	"net/http"
	"print-pro-backend/internal/api/handlers/auth_handler/shared"
	"print-pro-backend/internal/repositories"
	"print-pro-backend/internal/services/google_auth"
)

// CustomerAuthHandler handles customer-specific authentication requests
type CustomerAuthHandler struct {
	loginHandler    *LoginHandler
	googleHandler   *GoogleSignInHandler
	registerHandler *RegisterHandler
}

// NewCustomerAuthHandler creates and initializes a new CustomerAuthHandler instance
func NewCustomerAuthHandler(
	accountRepository *repositories.AccountRepository,
	customerProfileRepository *repositories.CustomerProfileRepository,
	googleAuthService *google_auth.GoogleAuthService,
	tokenHelper shared.TokenHelper,
	sendErrorResponse func(http.ResponseWriter, int, string, string),
	sendJSONResponse func(http.ResponseWriter, int, interface{}),
) *CustomerAuthHandler {
	// Initialize customer handlers
	loginHandler := NewLoginHandler(
		accountRepository,
		tokenHelper,
		sendErrorResponse,
		sendJSONResponse,
	)
	
	googleHandler := NewGoogleSignInHandler(
		googleAuthService,
		tokenHelper,
		sendErrorResponse,
		sendJSONResponse,
	)
	
	registerHandler := NewRegisterHandler(
		accountRepository,
		customerProfileRepository,
		tokenHelper,
		sendErrorResponse,
		sendJSONResponse,
	)
	
	return &CustomerAuthHandler{
		loginHandler:    loginHandler,
		googleHandler:   googleHandler,
		registerHandler: registerHandler,
	}
}

// HandleLoginCustomer delegates to customer login handler
func (h *CustomerAuthHandler) HandleLoginCustomer(w http.ResponseWriter, r *http.Request) {
	h.loginHandler.HandleLogin(w, r)
}

// HandleGoogleSignInCustomer delegates to customer Google sign-in handler
func (h *CustomerAuthHandler) HandleGoogleSignInCustomer(w http.ResponseWriter, r *http.Request) {
	h.googleHandler.HandleGoogleSignIn(w, r)
}

// HandleRegisterCustomer delegates to customer register handler
func (h *CustomerAuthHandler) HandleRegisterCustomer(w http.ResponseWriter, r *http.Request) {
	h.registerHandler.HandleRegister(w, r)
}

// LoginCustomer is an HTTP handler function for customer login
// This is the wrapper method that should be used in routes
func LoginCustomer(handler *CustomerAuthHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		handler.HandleLoginCustomer(w, r)
	}
}

// GoogleSignInCustomer is an HTTP handler function for customer Google sign-in
// This is the wrapper method that should be used in routes
func GoogleSignInCustomer(handler *CustomerAuthHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		handler.HandleGoogleSignInCustomer(w, r)
	}
}

// RegisterCustomer is an HTTP handler function for customer registration
// This is the wrapper method that should be used in routes
func RegisterCustomer(handler *CustomerAuthHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		handler.HandleRegisterCustomer(w, r)
	}
}

