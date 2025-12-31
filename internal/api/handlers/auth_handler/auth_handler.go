package auth_handler

import (
	"net/http"
	"print-pro-backend/internal/api/handlers/auth_handler/session_handlers"
	"print-pro-backend/internal/api/handlers/auth_handler/customer"
	"print-pro-backend/internal/api/handlers/auth_handler/partner"
	"print-pro-backend/internal/config"
	"print-pro-backend/internal/infrastructure"
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
	partnerLoginHandler   *partner.LoginHandler
	partnerGoogleHandler  *partner.GoogleSignInHandler
	customerLoginHandler  *customer.LoginHandler
	customerGoogleHandler *customer.GoogleSignInHandler
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
		refreshTokenHandler := session_handlers.NewRefreshTokenHandler(jwtService, sessionService)
		logoutHandler := session_handlers.NewLogoutHandler(sessionService, cfg)
		meHandler := session_handlers.NewMeHandler()
	
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
	
	// Initialize partner handlers with proper function references
	handler.partnerLoginHandler = partner.NewLoginHandler(
		accountRepository,
		tokenHelper,
		handler.sendErrorResponse,
		handler.sendJSONResponse,
	)
	
	handler.partnerGoogleHandler = partner.NewGoogleSignInHandler(
		googleAuthService,
		tokenHelper,
		handler.sendErrorResponse,
		handler.sendJSONResponse,
	)
	
	// Initialize customer handlers with proper function references
	handler.customerLoginHandler = customer.NewLoginHandler(
		accountRepository,
		tokenHelper,
		handler.sendErrorResponse,
		handler.sendJSONResponse,
	)
	
	handler.customerGoogleHandler = customer.NewGoogleSignInHandler(
		googleAuthService,
		tokenHelper,
		handler.sendErrorResponse,
		handler.sendJSONResponse,
	)
	
	return handler
}

// LoginPartner delegates to partner login handler
func (h *AuthHandler) LoginPartner(w http.ResponseWriter, r *http.Request) {
	h.partnerLoginHandler.HandleLogin(w, r)
}

// LoginCustomer delegates to customer login handler
func (h *AuthHandler) LoginCustomer(w http.ResponseWriter, r *http.Request) {
	h.customerLoginHandler.HandleLogin(w, r)
}

// GoogleSignInPartner delegates to partner Google sign-in handler
func (h *AuthHandler) GoogleSignInPartner(w http.ResponseWriter, r *http.Request) {
	h.partnerGoogleHandler.HandleGoogleSignIn(w, r)
}

// GoogleSignInCustomer delegates to customer Google sign-in handler
func (h *AuthHandler) GoogleSignInCustomer(w http.ResponseWriter, r *http.Request) {
	h.customerGoogleHandler.HandleGoogleSignIn(w, r)
}

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

// CheckUserType delegates to me handler
func (h *AuthHandler) CheckUserType(w http.ResponseWriter, r *http.Request) {
	h.meHandler.HandleCheckUserType(w, r, h.sendJSONResponse, h.sendErrorResponse)
}

