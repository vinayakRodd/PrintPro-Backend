package auth_middleware

import (
	"context"
	"log"
	"net/http"
	"strings"
	"time"
	"print-pro-backend/internal/models"
	"print-pro-backend/internal/services/jwt"
)

// OptionalAuthMiddleware verifies JWT token if present, but doesn't fail if missing
// This allows backward compatibility with Python agent (no auth) and Go agent (with JWT)
// If JWT is provided and valid, user is attached to context for logging/auditing
func OptionalAuthMiddleware(jwtService *jwt.JWTService) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			// Get access token from Authorization header
			authHeader := r.Header.Get("Authorization")
			
			// If no auth header, proceed without authentication (backward compatibility)
			if authHeader == "" {
				log.Printf("INFO: Partner agent request without authentication (backward compatibility mode)")
				next(w, r)
				return
			}

			// Extract token from "Bearer <token>"
			parts := strings.Split(authHeader, " ")
			if len(parts) != 2 || parts[0] != "Bearer" {
				log.Printf("WARNING: Invalid authorization header format - proceeding without auth")
				next(w, r)
				return
			}

			accessToken := parts[1]

			// Validate JWT access token
			log.Printf("🔍 AUTH: Validating access token for partner agent...")
			claims, err := jwtService.ValidateAccessToken(accessToken)
			if err != nil {
				log.Printf("❌ AUTH: Access token validation failed - %v (proceeding without auth for backward compatibility)", err)
				next(w, r)
				return
			}
			
			log.Printf("✅ AUTH: Access token validated successfully for user: %s (type: %s)", claims.UserID, claims.UserType)

			// Verify user is a partner
			if claims.UserType != "partner" {
				log.Printf("WARNING: Non-partner user attempting to use partner agent endpoint - proceeding without auth")
				next(w, r)
				return
			}

			// Create user object from JWT claims
			user := &models.User{
				ID:        claims.UserID,
				Email:     claims.Email,
				UserType:  claims.UserType,
				Provider:  "jwt",
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}

			// Attach user to request context (for logging/auditing)
			ctx := context.WithValue(r.Context(), "user", user)
			r = r.WithContext(ctx)

			// Call the next handler
			next(w, r)
		}
	}
}
