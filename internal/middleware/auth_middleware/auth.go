package auth_middleware

import (
	"context"
	"log"
	"net/http"
	"strings"
	"time"
	"print-pro-backend/internal/models"
	"print-pro-backend/internal/services/jwt"
	"print-pro-backend/internal/services/session"
)

// AuthMiddleware verifies the user's JWT access token from Authorization header
func AuthMiddleware(sessionService *session.SessionService, jwtService *jwt.JWTService) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			// Get access token from Authorization header
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte(`{"success": false, "message": "Unauthorized", "error": "No authorization header found"}`))
				return
			}

			// Extract token from "Bearer <token>"
			parts := strings.Split(authHeader, " ")
			if len(parts) != 2 || parts[0] != "Bearer" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte(`{"success": false, "message": "Unauthorized", "error": "Invalid authorization header format"}`))
				return
			}

			accessToken := parts[1]
			// SECURITY: Never log the actual token value - only log validation status

			// Validate JWT access token
			log.Printf("🔍 AUTH: Validating access token...")
			claims, err := jwtService.ValidateAccessToken(accessToken)
			if err != nil {
				log.Printf("❌ AUTH: Access token validation failed - %v", err)
				log.Printf("💡 AUTH: Access token expired or invalid - frontend should call /api/auth/refresh")
				
				// Set header to indicate frontend should refresh token (not logout)
				w.Header().Set("X-Token-Expired", "true")
				w.Header().Set("X-Should-Refresh", "true")
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte(`{"success": false, "message": "Unauthorized", "error": "Invalid or expired access token", "code": "ACCESS_TOKEN_EXPIRED"}`))
				return
			}
			
			log.Printf("✅ AUTH: Access token validated successfully - UserType: %s", claims.UserType)

			// Create user object from JWT claims
			user := &models.User{
				ID:        claims.UserID,
				Email:     claims.Email,
				UserType:  claims.UserType, // Store user type from JWT claims
				Provider:  "jwt",
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}

			// Attach user to request context
			ctx := context.WithValue(r.Context(), "user", user)
			r = r.WithContext(ctx)

			// Call the next handler
			next(w, r)
		}
	}
}

// GetUserFromContext retrieves the user from the request context
func GetUserFromContext(r *http.Request) (*models.User, bool) {
	user, ok := r.Context().Value("user").(*models.User)
	return user, ok
}

