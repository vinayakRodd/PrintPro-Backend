package auth_handler

import (
	"net/http"
	"print-pro-backend/internal/api/handlers/auth_handler/session_handlers"
	"print-pro-backend/internal/api/handlers/auth_handler/customer"
	"print-pro-backend/internal/api/handlers/auth_handler/partner"
	"print-pro-backend/internal/api/handlers/auth_handler/login"
	"print-pro-backend/internal/api/handlers/auth_handler/shared"
	"print-pro-backend/internal/config"
	"print-pro-backend/internal/infrastructure"
	"print-pro-backend/internal/middleware/auth_middleware"
	"print-pro-backend/internal/repositories"
	"print-pro-backend/internal/services/email"
	"print-pro-backend/internal/services/google_auth"
	"print-pro-backend/internal/services/jwt"
	"print-pro-backend/internal/services/otp"
	"print-pro-backend/internal/services/session"
)

// AuthHandler handles authentication-related requests
type AuthHandler struct {
	googleAuthService         *google_auth.GoogleAuthService
	sessionService            *session.SessionService
	emailService              *email.EmailService
	otpService                *otp.OTPService
	userRepository            *repositories.UserRepository
	accountRepository         *repositories.AccountRepository
	partnerProfileRepository  *repositories.PartnerProfileRepository
	customerProfileRepository *repositories.CustomerProfileRepository
	postgresClient            *infrastructure.PostgresClient
	redisClient               *infrastructure.RedisClient
	jwtService                *jwt.JWTService
	config                    *config.Config
	
	// Modular handlers
	tokenHelper           *session_handlers.TokenHelper
	refreshTokenHandler   *session_handlers.RefreshTokenHandler
	logoutHandler         *session_handlers.LogoutHandler
	meHandler             *session_handlers.MeHandler
	// Partner handlers (exposed for direct access from routes)
	PartnerAuthHandler *partner.PartnerAuthHandler
	// Customer handlers (exposed for direct access from routes)
	CustomerAuthHandler *customer.CustomerAuthHandler
	// Shared handlers (work for both customer and partner)
	emailHandler          *shared.EmailHandler
	forgotPasswordHandler *shared.ForgotPasswordHandler
	otpHandler            *shared.OTPHandler
	resetPasswordHandler  *shared.ResetPasswordHandler
	// Legacy login handlers (deprecated)
	legacyLoginHandler      *login.LoginHandler
	legacyRegisterHandler   *login.RegisterHandler
	legacyGoogleSignInHandler *login.GoogleSignInHandler
}

// NewAuthHandler creates a new AuthHandler instance
func NewAuthHandler(
	googleAuthService *google_auth.GoogleAuthService,
	sessionService *session.SessionService,
	emailService *email.EmailService,
	otpService *otp.OTPService,
	userRepository *repositories.UserRepository,
	accountRepository *repositories.AccountRepository,
	partnerProfileRepository *repositories.PartnerProfileRepository,
	customerProfileRepository *repositories.CustomerProfileRepository,
	postgresClient *infrastructure.PostgresClient,
	redisClient *infrastructure.RedisClient,
	jwtService *jwt.JWTService,
	cfg *config.Config,
) *AuthHandler {
		// Initialize session handlers
		tokenHelper := session_handlers.NewTokenHelper(jwtService, sessionService, cfg)
		refreshTokenHandler := session_handlers.NewRefreshTokenHandler(jwtService, sessionService, tokenHelper, cfg, partnerProfileRepository, redisClient)
		logoutHandler := session_handlers.NewLogoutHandler(sessionService, cfg)
		meHandler := session_handlers.NewMeHandler(accountRepository)
	
	handler := &AuthHandler{
		googleAuthService:         googleAuthService,
		sessionService:             sessionService,
		emailService:               emailService,
		otpService:                 otpService,
		userRepository:             userRepository,
		accountRepository:          accountRepository,
		partnerProfileRepository:   partnerProfileRepository,
		customerProfileRepository:  customerProfileRepository,
		postgresClient:             postgresClient,
		redisClient:                redisClient,
		jwtService:                 jwtService,
		config:                     cfg,
		tokenHelper:                tokenHelper,
		refreshTokenHandler:        refreshTokenHandler,
		logoutHandler:              logoutHandler,
		meHandler:                  meHandler,
	}
	
	// Initialize partner handlers (initialization moved to partner package)
	handler.PartnerAuthHandler = partner.NewPartnerAuthHandler(
		accountRepository,
		partnerProfileRepository,
		googleAuthService,
		otpService,
		emailService,
		tokenHelper,
		handler.sendErrorResponse,
		handler.sendJSONResponse,
	)
	
	// Initialize customer handlers (initialization moved to customer package)
	handler.CustomerAuthHandler = customer.NewCustomerAuthHandler(
		accountRepository,
		customerProfileRepository,
		googleAuthService,
		tokenHelper,
		handler.sendErrorResponse,
		handler.sendJSONResponse,
	)
	
	// Initialize shared handlers (work for both customer and partner)
	handler.emailHandler = shared.NewEmailHandler(
		otpService,
		userRepository,
		handler.sendErrorResponse,
		handler.sendJSONResponse,
	)
	
	handler.forgotPasswordHandler = shared.NewForgotPasswordHandler(
		otpService,
		emailService,
		handler.sendErrorResponse,
		handler.sendJSONResponse,
	)
	
	handler.otpHandler = shared.NewOTPHandler(
		otpService,
		handler.sendErrorResponse,
		handler.sendJSONResponse,
	)
	
	handler.resetPasswordHandler = shared.NewResetPasswordHandler(
		otpService,
		accountRepository,
		handler.sendErrorResponse,
		handler.sendJSONResponse,
	)
	
	// Initialize legacy login handlers (deprecated)
	handler.legacyLoginHandler = login.NewLoginHandler(
		accountRepository,
		tokenHelper,
		handler.sendErrorResponse,
		handler.sendJSONResponse,
	)
	
	handler.legacyRegisterHandler = login.NewRegisterHandler(
		handler.sendErrorResponse,
	)
	
	handler.legacyGoogleSignInHandler = login.NewGoogleSignInHandler(
		googleAuthService,
		tokenHelper,
		handler.sendErrorResponse,
		handler.sendJSONResponse,
	)
	
	return handler
}

// Partner and customer handler methods are now in their respective packages
// Access them via: partner.LoginPartner(authHandler.PartnerAuthHandler)
// or customer.LoginCustomer(authHandler.CustomerAuthHandler)

// RefreshToken delegates to refresh token handler
func (h *AuthHandler) RefreshToken(w http.ResponseWriter, r *http.Request) {
	h.refreshTokenHandler.HandleRefreshToken(w, r, h.sendJSONResponse, h.sendErrorResponse)
}

// Logout delegates to logout handler
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	h.logoutHandler.HandleLogout(w, r, h.sendJSONResponse, h.sendErrorResponse)
}

// GetMe delegates to me handler
func (h *AuthHandler) GetMe(w http.ResponseWriter, r *http.Request) {
	h.meHandler.HandleGetMe(w, r, h.sendJSONResponse, h.sendErrorResponse)
}

// GetPartnerMe returns the partner profile information for the authenticated partner
func (h *AuthHandler) GetPartnerMe(w http.ResponseWriter, r *http.Request) {
	// Only allow GET requests
	if r.Method != http.MethodGet {
		h.sendErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed", "Only GET method is allowed")
		return
	}

	// Get authenticated user from context
	user, ok := auth_middleware.GetUserFromContext(r)
	if !ok {
		h.sendErrorResponse(w, http.StatusUnauthorized, "Unauthorized", "User not found in session")
		return
	}

	// Only partners can access this endpoint
	if user.UserType != "partner" {
		h.sendErrorResponse(w, http.StatusForbidden, "Forbidden", "This endpoint is only available for partners")
		return
	}

	ctx := r.Context()

	// Get partner profile using the user's email (user.ID is the email)
	partnerProfile, err := h.partnerProfileRepository.GetByAccountEmail(ctx, user.ID)
	if err != nil {
		h.sendErrorResponse(w, http.StatusNotFound, "Partner profile not found", "Partner profile not found for this account")
		return
	}

	// Return partner profile with email
	response := map[string]interface{}{
		"success": true,
		"message": "Partner profile retrieved successfully",
		"profile": map[string]interface{}{
			"partner_email": partnerProfile.PartnerEmail,
			"shop_name":     partnerProfile.ShopName,
			"printer_id":    partnerProfile.PrinterID,
			"status":        partnerProfile.Status,
		},
	}

	h.sendJSONResponse(w, http.StatusOK, response)
}

// CheckUserType delegates to me handler
func (h *AuthHandler) CheckUserType(w http.ResponseWriter, r *http.Request) {
	h.meHandler.HandleCheckUserType(w, r, h.sendJSONResponse, h.sendErrorResponse)
}

// Login handles user login (legacy unified endpoint - deprecated)
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	h.legacyLoginHandler.HandleLogin(w, r)
}

// Register handles user registration (legacy - deprecated)
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	h.legacyRegisterHandler.HandleRegister(w, r)
}

// GoogleSignIn handles Google Sign-In (legacy unified endpoint - deprecated)
func (h *AuthHandler) GoogleSignIn(w http.ResponseWriter, r *http.Request) {
	h.legacyGoogleSignInHandler.HandleGoogleSignIn(w, r)
}

// Partner and customer register methods are now in their respective packages
// Access them via: partner.RegisterPartner(authHandler.PartnerAuthHandler)
// or customer.RegisterCustomer(authHandler.CustomerAuthHandler)

// GetEmail delegates to email handler
func (h *AuthHandler) GetEmail(w http.ResponseWriter, r *http.Request) {
	h.emailHandler.HandleGetEmail(w, r)
}

// ForgotPassword delegates to forgot password handler
func (h *AuthHandler) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	h.forgotPasswordHandler.HandleForgotPassword(w, r)
}

// VerifyOTP delegates to OTP handler
func (h *AuthHandler) VerifyOTP(w http.ResponseWriter, r *http.Request) {
	h.otpHandler.HandleVerifyOTP(w, r)
}

// ResetPassword delegates to reset password handler
func (h *AuthHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	h.resetPasswordHandler.HandleResetPassword(w, r)
}

