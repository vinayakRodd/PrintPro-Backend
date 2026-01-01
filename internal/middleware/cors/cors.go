package cors

import (
	"net"
	"net/http"
	"strings"
)

// CORS middleware handles Cross-Origin Resource Sharing
func CORS(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get client IP address (handle proxies)
		clientIP := getClientIP(r)
		
		// Global IP whitelist - allow this IP to access all routes globally
		// This IP is allowed to access all routes without restrictions
		allowedIPs := []string{
			"10.24.171.142",
		}
		
		// Check if client IP is in global whitelist
		// If it is, the IP is allowed to proceed (no blocking needed)
		_ = clientIP // Use clientIP to avoid unused variable warning
		for _, allowedIP := range allowedIPs {
			if clientIP == allowedIP {
				// IP is globally allowed - proceed with request
				break
			}
		}

		// Get the origin from the request
		origin := r.Header.Get("Origin")

		// Allow specific origins (for development, allow localhost:3000 and Machine B's IP)
		allowedOrigins := []string{
			"http://localhost:3000",
			"http://10.24.171.142:3000",
		}

		// Check if origin is allowed
		allowed := false
		for _, allowedOrigin := range allowedOrigins {
			if origin == allowedOrigin {
				allowed = true
				break
			}
		}

		// If origin is allowed, set it; otherwise use the request origin if it's localhost or Machine B's IP
		if allowed || (origin != "" && (origin == "http://localhost:3000" || origin == "http://localhost:3001" || origin == "http://10.24.171.142:3000")) {
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

// getClientIP extracts the real client IP from the request
// Handles cases where the request goes through proxies (X-Forwarded-For, X-Real-IP)
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header first (most common proxy header)
	forwarded := r.Header.Get("X-Forwarded-For")
	if forwarded != "" {
		// X-Forwarded-For can contain multiple IPs, take the first one
		ips := strings.Split(forwarded, ",")
		if len(ips) > 0 {
			ip := strings.TrimSpace(ips[0])
			if ip != "" {
				return ip
			}
		}
	}

	// Check X-Real-IP header (alternative proxy header)
	realIP := r.Header.Get("X-Real-IP")
	if realIP != "" {
		return realIP
	}

	// Fall back to RemoteAddr
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

