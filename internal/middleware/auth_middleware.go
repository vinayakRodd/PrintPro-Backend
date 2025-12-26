package middleware

import (
	"context"
	"log"
	"net/http"
	"print-pro-backend/internal/models"
	"print-pro-backend/internal/services"
)

// AuthMiddleware verifies the user's session token from cookie
func AuthMiddleware(sessionService *services.SessionService) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			// Get session token from cookie
			cookie, err := r.Cookie("session_token")
			if err != nil || cookie.Value == "" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte(`{"success": false, "message": "Unauthorized", "error": "No session token found"}`))
				return
			}

			// Verify session in Redis
			ctx := r.Context()
			user, err := sessionService.GetSession(ctx, cookie.Value)
			if err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte(`{"success": false, "message": "Unauthorized", "error": "Invalid or expired session"}`))
				return
			}
			
			log.Printf("Session found in cache retrieved for user")

			// Add user to request context for use in handlers
			ctx = context.WithValue(ctx, "user", user)
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

