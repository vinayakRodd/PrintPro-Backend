package role

import (
	"log"
	"net/http"
	"strings"
)

// RequireRole creates middleware that requires a specific user role
func RequireRole(requiredRole string) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			// Get user_type from context (set by auth middleware)
			userType, ok := r.Context().Value("user_type").(string)
			if !ok || userType == "" {
				log.Printf("❌ ROLE: User type not found in context")
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				w.Write([]byte(`{"success": false, "message": "Forbidden", "error": "User role not found"}`))
				return
			}

			// Normalize roles for comparison
			userType = strings.ToLower(strings.TrimSpace(userType))
			requiredRole = strings.ToLower(strings.TrimSpace(requiredRole))

			// Check if user has the required role
			if userType != requiredRole {
				log.Printf("❌ ROLE: Access denied - User type: %s, Required: %s", userType, requiredRole)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				w.Write([]byte(`{"success": false, "message": "Forbidden", "error": "Insufficient permissions"}`))
				return
			}

			log.Printf("✅ ROLE: Access granted - User type: %s matches required: %s", userType, requiredRole)
			next(w, r)
		}
	}
}

