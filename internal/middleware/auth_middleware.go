package middleware

import (
	"context"
	"log"
	"net/http"
	"strings"
	"print-pro-backend/internal/models"
	"print-pro-backend/internal/services"
)

// AuthMiddleware verifies the user's JWT access token from Authorization header
func AuthMiddleware(sessionService *services.SessionService, jwtService *services.JWTService) func(http.HandlerFunc) http.HandlerFunc {
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

			// Validate JWT access token
			log.Printf("🔍 AUTH: Validating access token...")
			claims, err := jwtService.ValidateAccessToken(accessToken)
			if err != nil {
				log.Printf("❌ AUTH: Access token validation failed - %v", err)
				log.Printf("💡 AUTH: Access token expired or invalid - frontend should call /api/auth/refresh")
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte(`{"success": false, "message": "Unauthorized", "error": "Invalid or expired access token"}`))
				return
			}
			
			log.Printf("✅ AUTH: Access token validated successfully for user: %s", claims.UserID)

			// Create user object from JWT claims
			user := &models.User{
				ID:    claims.UserID,
				Email: claims.Email,
			}

			// Add user to request context for use in handlers
			ctx := context.WithValue(r.Context(), "user", user)
			r = r.WithContext(ctx)

			// Continue to next handler
			next(w, r)
		}
	}
}

// GetUserFromContext extracts the user from request context
func GetUserFromContext(r *http.Request) (*models.User, bool) {
	user, ok := r.Context().Value("user").(*models.User)
	return user, ok
}

