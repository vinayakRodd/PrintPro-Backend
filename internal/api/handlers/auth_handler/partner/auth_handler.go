package partner

import (
	"context"
	"net/http"
	"print-pro-backend/internal/api/handlers/auth_handler/shared"
	"print-pro-backend/internal/repositories"
	"print-pro-backend/internal/services/google_auth"
)

// PartnerAuthHandler handles partner-specific authentication requests
type PartnerAuthHandler struct {
	loginHandler    *LoginHandler
	googleHandler   *GoogleSignInHandler
	registerHandler *RegisterHandler
}

// NewPartnerAuthHandler creates and initializes a new PartnerAuthHandler instance
func NewPartnerAuthHandler(
	accountRepository *repositories.AccountRepository,
	partnerProfileRepository *repositories.PartnerProfileRepository,
	googleAuthService *google_auth.GoogleAuthService,
	otpService interface {
		GenerateOTP(ctx context.Context, email string) (string, error)
		MarkOTPEmailSent(ctx context.Context, email string) (bool, error)
	},
	emailService interface {
		SendOTPEmail(email, otp string) error
	},
	tokenHelper shared.TokenHelper,
	sendErrorResponse func(http.ResponseWriter, int, string, string),
	sendJSONResponse func(http.ResponseWriter, int, interface{}),
) *PartnerAuthHandler {
	// Initialize partner handlers
	loginHandler := NewLoginHandler(
		accountRepository,
		partnerProfileRepository,
		otpService,
		emailService,
		tokenHelper,
		sendErrorResponse,
		sendJSONResponse,
	)
	
	googleHandler := NewGoogleSignInHandler(
		googleAuthService,
		partnerProfileRepository,
		tokenHelper,
		sendErrorResponse,
		sendJSONResponse,
	)
	
	registerHandler := NewRegisterHandler(
		accountRepository,
		partnerProfileRepository,
		tokenHelper,
		sendErrorResponse,
		sendJSONResponse,
	)
	
	return &PartnerAuthHandler{
		loginHandler:    loginHandler,
		googleHandler:   googleHandler,
		registerHandler: registerHandler,
	}
}

// HandleLoginPartner delegates to partner login handler
func (h *PartnerAuthHandler) HandleLoginPartner(w http.ResponseWriter, r *http.Request) {
	h.loginHandler.HandleLogin(w, r)
}

// HandleGoogleSignInPartner delegates to partner Google sign-in handler
func (h *PartnerAuthHandler) HandleGoogleSignInPartner(w http.ResponseWriter, r *http.Request) {
	h.googleHandler.HandleGoogleSignIn(w, r)
}

// HandleRegisterPartner delegates to partner register handler
func (h *PartnerAuthHandler) HandleRegisterPartner(w http.ResponseWriter, r *http.Request) {
	h.registerHandler.HandleRegister(w, r)
}

// LoginPartner is an HTTP handler function for partner login
// This is the wrapper method that should be used in routes
func LoginPartner(handler *PartnerAuthHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		handler.HandleLoginPartner(w, r)
	}
}

// GoogleSignInPartner is an HTTP handler function for partner Google sign-in
// This is the wrapper method that should be used in routes
func GoogleSignInPartner(handler *PartnerAuthHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		handler.HandleGoogleSignInPartner(w, r)
	}
}

// RegisterPartner is an HTTP handler function for partner registration
// This is the wrapper method that should be used in routes
func RegisterPartner(handler *PartnerAuthHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		handler.HandleRegisterPartner(w, r)
	}
}

