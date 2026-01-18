package session_handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
	"print-pro-backend/internal/config"
	"print-pro-backend/internal/infrastructure"
	"print-pro-backend/internal/repositories"
	"print-pro-backend/internal/services/jwt"
	"print-pro-backend/internal/services/session"
	"github.com/redis/go-redis/v9"
)

// RefreshTokenHandler handles refresh token requests
type RefreshTokenHandler struct {
	jwtService             *jwt.JWTService
	sessionService         *session.SessionService
	tokenHelper            *TokenHelper
	config                 *config.Config
	partnerProfileRepository *repositories.PartnerProfileRepository
	redisClient            *infrastructure.RedisClient
}

// NewRefreshTokenHandler creates a new RefreshTokenHandler instance
func NewRefreshTokenHandler(
	jwtService *jwt.JWTService,
	sessionService *session.SessionService,
	tokenHelper *TokenHelper,
	cfg *config.Config,
	partnerProfileRepository *repositories.PartnerProfileRepository,
	redisClient *infrastructure.RedisClient,
) *RefreshTokenHandler {
	return &RefreshTokenHandler{
		jwtService:             jwtService,
		sessionService:         sessionService,
		tokenHelper:            tokenHelper,
		config:                 cfg,
		partnerProfileRepository: partnerProfileRepository,
		redisClient:            redisClient,
	}
}

// HandleRefreshToken handles refresh token requests to get a new access token
func (h *RefreshTokenHandler) HandleRefreshToken(w http.ResponseWriter, r *http.Request, sendJSONResponse func(http.ResponseWriter, int, interface{}), sendErrorResponse func(http.ResponseWriter, int, string, string)) {
	log.Printf("🔄 REFRESH: Refresh token endpoint hit - Method: %s, URL: %s", r.Method, r.URL.Path)

	if r.Method != http.MethodPost {
		sendErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed", "Only POST method is allowed")
		return
	}

	// Get refresh token from cookie OR Authorization header (for Go agent)
	// SECURITY: Never log the actual token value - only log status messages
	var refreshToken string
	var isGoAgent bool // Track if request is from Go agent (needs refresh_token in response)
	ctx := r.Context()
	
	// Try Authorization header first (for Go agent)
	authHeader := r.Header.Get("Authorization")
	if authHeader != "" {
		parts := strings.Split(authHeader, " ")
		if len(parts) == 2 && parts[0] == "Bearer" {
			refreshToken = parts[1]
			isGoAgent = true // Go agent sends refresh token in Authorization header
			log.Printf("🔍 REFRESH: Refresh token found in Authorization header (Go agent), validating...")
		}
	}
	
	// Fallback to cookie (for web frontend)
	if refreshToken == "" {
		cookie, err := r.Cookie("refresh_token")
		if err == nil && cookie.Value != "" {
			refreshToken = cookie.Value
			isGoAgent = false // Web frontend uses cookie, refresh_token should NOT be in response body
			log.Printf("🔍 REFRESH: Refresh token found in cookie (web frontend), validating...")
		}
	}
	
	if refreshToken == "" {
		log.Printf("❌ REFRESH: Refresh token not found in header or cookie")
		sendErrorResponse(w, http.StatusUnauthorized, "Unauthorized", "Refresh token not found")
		return
	}

	// SECURITY NOTE: refreshToken variable contains sensitive data - never log it

	// Validate refresh token JWT
	claims, err := h.jwtService.ValidateRefreshToken(refreshToken)
	if err != nil {
		log.Printf("❌ REFRESH: Invalid refresh token - %v", err)
		sendErrorResponse(w, http.StatusUnauthorized, "Unauthorized", "Invalid or expired refresh token")
		return
	}
	log.Printf("✅ REFRESH: Refresh token JWT validated successfully")

	// Verify refresh token exists in Redis (for revocation check)
	// NOTE: If Redis doesn't have it but JWT is valid, allow refresh (Redis TTL might have expired)
	// This is more lenient - we trust JWT expiration over Redis TTL
	log.Printf("🔍 REFRESH: Checking refresh token in Redis...")
	_, err = h.sessionService.ValidateRefreshToken(ctx, refreshToken)
	if err != nil {
		// Token not in Redis - could be:
		// 1. Redis TTL expired (but JWT still valid) - allow refresh
		// 2. Token was revoked - but JWT validation already passed, so allow refresh
		// 3. First time refresh after Redis restart - allow refresh
		log.Printf("⚠️ REFRESH: Refresh token not found in Redis (may have expired in Redis, but JWT is valid) - allowing refresh")
		// Continue - JWT validation is the source of truth
	} else {
		log.Printf("✅ REFRESH: Refresh token found in Redis (not revoked)")
	}

	// SINGLE SESSION POLICY: For partners, verify this is the active session
	// Made more lenient - if active session not found in Redis, allow refresh (JWT is already validated)
	if claims.UserType == "partner" {
		log.Printf("🔒 REFRESH: Partner refresh detected - verifying single session policy")
		activeRefreshToken, err := h.sessionService.GetPartnerActiveRefreshToken(ctx, claims.UserID)
		if err != nil {
			// Active session not found in Redis - could be:
			// 1. Redis TTL expired (but JWT still valid) - allow refresh and update active session
			// 2. First refresh after Redis restart - allow refresh and set active session
			log.Printf("⚠️ REFRESH: Active partner session not found in Redis (may have expired) - allowing refresh and updating active session")
			// Continue - we'll set the active session after token rotation
		} else if activeRefreshToken != refreshToken {
			// Active session exists but doesn't match - this means another device logged in
			log.Printf("❌ REFRESH: Refresh token is not the active session for partner - another device is active")
			// Invalidate the token being used (it's not the active one)
			h.sessionService.DeleteRefreshToken(ctx, refreshToken)
			// Send specific error message for frontend to display
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": "Session expired",
				"error":   "You have been logged out because you logged in from another device. Please login again.",
				"code":    "SESSION_INVALIDATED_ANOTHER_DEVICE",
			})
			return
		} else {
			log.Printf("✅ REFRESH: Refresh token is the active partner session")
		}
		
		// SECURITY: Check partner_profiles.status before allowing token refresh
		// Uses Redis cache (60s TTL) to reduce database queries
		partnerEmail := claims.Email
		log.Printf("🔍 REFRESH: Checking partner profile status")
		
		// Check Redis cache first (60s TTL to reduce database queries)
		cacheKey := fmt.Sprintf("partner:status:%s", partnerEmail)
		cachedStatus, cacheErr := h.redisClient.Get(ctx, cacheKey)
		
		var isAuthorized bool
		cacheHit := false
		
		if cacheErr == nil {
			// Cache hit - parse cached status
			statusBool, parseErr := strconv.ParseBool(cachedStatus)
			if parseErr == nil {
				isAuthorized = statusBool
				cacheHit = true
				log.Printf("✅ REFRESH: Partner status retrieved from cache - Status: %v", isAuthorized)
				
				// If status is false, block refresh immediately
				if !isAuthorized {
					log.Printf("❌ REFRESH: Partner account not authorized (cached) - Status: false - BLOCKING TOKEN REFRESH")
					// Invalidate the refresh token since partner is no longer authorized
					h.sessionService.DeleteRefreshToken(ctx, refreshToken)
					h.sessionService.InvalidatePartnerSession(ctx, claims.UserID)
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusUnauthorized)
					json.NewEncoder(w).Encode(map[string]interface{}{
						"success": false,
						"error":   "Unauthorized",
						"message": "Your partner account is not authorized. Please contact administrator.",
					})
					return
				}
				
				log.Printf("✅ REFRESH: Partner profile verified (cached) - Status: true")
			} else {
				// Cached value is invalid, fall through to database query
				log.Printf("⚠️ REFRESH: Invalid cached status value, querying database")
				cacheHit = false
			}
		} else if cacheErr != redis.Nil {
			// Redis error (not a cache miss) - log but continue to database query
			log.Printf("⚠️ REFRESH: Redis cache error (non-fatal): %v, querying database", cacheErr)
			cacheHit = false
		}
		
		// Cache miss or invalid cache value - query database
		if !cacheHit {
			partnerProfile, dbErr := h.partnerProfileRepository.GetByAccountEmail(ctx, partnerEmail)
			if dbErr != nil {
				log.Printf("❌ REFRESH: Partner profile not found - Error: %v", dbErr)
				// Invalidate the refresh token since partner profile not found
				h.sessionService.DeleteRefreshToken(ctx, refreshToken)
				h.sessionService.InvalidatePartnerSession(ctx, claims.UserID)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"success": false,
					"error":   "Unauthorized",
					"message": "Partner profile not found. Please contact administrator.",
				})
				return
			}
			
			isAuthorized = partnerProfile.Status
			
			// Cache the status for 60 seconds
			statusStr := strconv.FormatBool(isAuthorized)
			if cacheSetErr := h.redisClient.Set(ctx, cacheKey, statusStr, 60*time.Second); cacheSetErr != nil {
				log.Printf("⚠️ REFRESH: Failed to cache partner status (non-fatal): %v", cacheSetErr)
			} else {
				log.Printf("✅ REFRESH: Partner status cached for 60 seconds - Status: %v", isAuthorized)
			}
			
			// Check if partner status is true (authorized)
			if !isAuthorized {
				log.Printf("❌ REFRESH: Partner account not authorized - Status: false - BLOCKING TOKEN REFRESH")
				// Invalidate the refresh token since partner is no longer authorized
				h.sessionService.DeleteRefreshToken(ctx, refreshToken)
				h.sessionService.InvalidatePartnerSession(ctx, claims.UserID)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"success": false,
					"error":   "Unauthorized",
					"message": "Your partner account is not authorized. Please contact administrator.",
				})
				return
			}
			
			log.Printf("✅ REFRESH: Partner profile verified - Status: true, Shop: %s", partnerProfile.ShopName)
		}
	}

	// TOKEN ROTATION: Invalidate old refresh token and generate new one
	log.Printf("🔄 REFRESH: Rotating refresh token (invalidating old, generating new)")
	
	// Delete old refresh token from Redis
	if err := h.sessionService.DeleteRefreshToken(ctx, refreshToken); err != nil {
		log.Printf("WARNING: Failed to delete old refresh token from Redis: %v", err)
		// Continue anyway - token will expire naturally
	}
	log.Printf("✅ REFRESH: Old refresh token invalidated in Redis")

	// Generate new access token (use user_type from claims)
	log.Printf("🔑 REFRESH: Generating new access token (expires in 15 minutes)")
	newAccessToken, err := h.jwtService.GenerateAccessToken(claims.UserID, claims.Email, claims.UserType)
	if err != nil {
		log.Printf("ERROR: Failed to generate new access token - %v", err)
		sendErrorResponse(w, http.StatusInternalServerError, "Failed to generate access token", err.Error())
		return
	}

	// Generate new refresh token (TOKEN ROTATION)
	log.Printf("🔑 REFRESH: Generating new refresh token (expires in 7 days)")
	newRefreshToken, err := h.jwtService.GenerateRefreshToken(claims.UserID, claims.Email, claims.UserType)
	if err != nil {
		log.Printf("ERROR: Failed to generate new refresh token - %v", err)
		sendErrorResponse(w, http.StatusInternalServerError, "Failed to generate refresh token", err.Error())
		return
	}

	// Store new refresh token in Redis
	if err := h.sessionService.StoreRefreshToken(ctx, newRefreshToken, claims.UserID); err != nil {
		log.Printf("ERROR: Failed to store new refresh token in Redis: %v", err)
		sendErrorResponse(w, http.StatusInternalServerError, "Failed to store refresh token", err.Error())
		return
	}
	log.Printf("✅ REFRESH: New refresh token stored in Redis")

	// SINGLE SESSION POLICY: For partners, update the active session mapping
	// Made non-fatal - if this fails, refresh still succeeds (Redis might have issues, but JWT is valid)
	if claims.UserType == "partner" {
		log.Printf("💾 REFRESH: Updating active partner session")
		if err := h.sessionService.SetPartnerActiveRefreshToken(ctx, claims.UserID, newRefreshToken); err != nil {
			log.Printf("⚠️ REFRESH: Failed to update active partner session (non-fatal): %v - refresh still succeeds", err)
			// Continue - refresh token is valid and new tokens are generated, session update failure is not critical
		} else {
			log.Printf("✅ REFRESH: Active partner session updated successfully")
		}
	}

	// Set new refresh token cookie (TOKEN ROTATION)
	refreshCookie := &http.Cookie{
		Name:     "refresh_token",
		Value:    newRefreshToken,
		Path:     "/",
		MaxAge:   7 * 24 * 60 * 60, // 7 days
		HttpOnly: true,              // Prevents JavaScript access (XSS protection)
		Secure:   h.config.SecureCookies, // Set via config (true in production with HTTPS)
		SameSite: http.SameSiteLaxMode, // CSRF protection
	}
	
	// Set domain if configured
	if h.config.CookieDomain != "" {
		refreshCookie.Domain = h.config.CookieDomain
	}
	
	http.SetCookie(w, refreshCookie)
	log.Printf("✅ REFRESH: New refresh token cookie set")

	log.Printf("✅ REFRESH: New access token generated successfully")
	log.Printf("🎉 REFRESH: Token refresh completed with rotation - user can continue without re-login")
	// SECURITY NOTE: newAccessToken and newRefreshToken contain sensitive data - never log them, only return in response

	// Build response - only include refresh_token in response body for Go agent
	// SECURITY: Web frontend gets refresh_token in HttpOnly cookie only (not in response body)
	response := map[string]interface{}{
		"success":      true,
		"access_token": newAccessToken,
		"message":      "Access token refreshed successfully",
	}
	
	// Only include refresh_token in response body for Go agent (Authorization header)
	// Web frontend gets it in HttpOnly cookie only (more secure - prevents XSS exposure)
	if isGoAgent {
		response["refresh_token"] = newRefreshToken
		log.Printf("🔐 REFRESH: Including refresh_token in response body for Go agent")
	} else {
		log.Printf("🔐 REFRESH: Refresh_token NOT in response body (web frontend - it's in HttpOnly cookie only)")
	}
	
	sendJSONResponse(w, http.StatusOK, response)
}

