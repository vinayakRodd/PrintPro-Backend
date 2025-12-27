package cors

import (
	"net/http"
)

// CORS middleware handles Cross-Origin Resource Sharing
func CORS(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get the origin from the request
		origin := r.Header.Get("Origin")

		// Allow specific origins (for development, allow localhost:3000)
		allowedOrigins := []string{
			"http://localhost:3000",
		}

		// Check if origin is allowed
		allowed := false
		for _, allowedOrigin := range allowedOrigins {
			if origin == allowedOrigin {
				allowed = true
				break
			}
		}

		// If origin is allowed, set it; otherwise use the request origin if it's localhost
		if allowed || (origin != "" && (origin == "http://localhost:3000" || origin == "http://localhost:3001")) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		} else if origin != "" {
			// For other origins, you might want to restrict this in production
			w.Header().Set("Access-Control-Allow-Origin", origin)
		}

		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Credentials", "true")

		// Handle preflight OPTIONS requests
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		// Call the next handler
		next(w, r)
	}
}

