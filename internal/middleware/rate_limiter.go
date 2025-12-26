package middleware

import (
	"fmt"
	"net/http"
	"print-pro-backend/internal/infrastructure"
	"strconv"
	"time"
)

// RateLimiter handles rate limiting using Redis
type RateLimiter struct {
	redisClient *infrastructure.RedisClient
	maxRequests int           // Maximum requests per window
	windowSize  time.Duration // Time window (e.g., 1 minute)
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(redisClient *infrastructure.RedisClient, maxRequests int, windowSize time.Duration) *RateLimiter {
	return &RateLimiter{
		redisClient: redisClient,
		maxRequests: maxRequests,
		windowSize:  windowSize,
	}
}

// LimitMiddleware returns a middleware function for rate limiting
func (rl *RateLimiter) LimitMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get client identifier (IP address or user ID)
		clientID := rl.getClientID(r)
		
		// Create rate limit key
		key := fmt.Sprintf("ratelimit:%s:%d", clientID, time.Now().Unix()/int64(rl.windowSize.Seconds()))
		
		ctx := r.Context()
		
		// Increment counter
		count, err := rl.redisClient.Increment(ctx, key)
		if err != nil {
			// If Redis fails, allow the request (fail open)
			next(w, r)
			return
		}
		
		// Set expiration on first request
		if count == 1 {
			rl.redisClient.SetWithExpiration(r.Context(), key, count, rl.windowSize)
		}
		
		// Check if limit exceeded
		if count > int64(rl.maxRequests) {
			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(rl.maxRequests))
			w.Header().Set("X-RateLimit-Remaining", "0")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			fmt.Fprintf(w, `{"success": false, "message": "Rate limit exceeded", "error": "Too many requests. Please try again later."}`)
			return
		}
		
		// Set rate limit headers
		remaining := rl.maxRequests - int(count)
		if remaining < 0 {
			remaining = 0
		}
		w.Header().Set("X-RateLimit-Limit", strconv.Itoa(rl.maxRequests))
		w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
		
		// Continue to next handler
		next(w, r)
	}
}

// getClientID extracts client identifier from request
func (rl *RateLimiter) getClientID(r *http.Request) string {
	// Try to get from X-Forwarded-For header (for proxies)
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		return forwarded
	}
	
	// Fall back to RemoteAddr
	return r.RemoteAddr
}

