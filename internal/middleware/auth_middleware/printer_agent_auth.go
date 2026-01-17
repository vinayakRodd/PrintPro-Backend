package auth_middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
	"print-pro-backend/internal/infrastructure"
	"print-pro-backend/internal/models"
	"print-pro-backend/internal/services/jwt"
	"print-pro-backend/internal/repositories"
	"github.com/redis/go-redis/v9"
)

// OptionalAuthMiddleware verifies JWT token if present, but doesn't fail if missing
// This allows backward compatibility with Python agent (no auth) and Go agent (with JWT)
// If JWT is provided and valid, checks partner_profiles.status (cached in Redis with 60s TTL) and blocks if status is false
func OptionalAuthMiddleware(jwtService *jwt.JWTService, partnerProfileRepository *repositories.PartnerProfileRepository, redisClient *infrastructure.RedisClient) func(http.HandlerFunc) http.HandlerFunc {
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
			// IMPORTANT: If JWT is present, it MUST be valid (no backward compatibility for invalid JWTs)
			// This ensures status checks are enforced when agents send JWTs
			log.Printf("🔍 AUTH: Validating access token for partner agent...")
			claims, err := jwtService.ValidateAccessToken(accessToken)
			if err != nil {
				log.Printf("❌ AUTH: Access token validation failed - %v - BLOCKING REQUEST (JWT present but invalid)", err)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"success": false,
					"error":   "Unauthorized",
					"message": "Invalid or expired access token. Please login again.",
				})
				return
			}
			
			log.Printf("✅ AUTH: Access token validated successfully for user: %s (type: %s)", claims.UserID, claims.UserType)

			// Verify user is a partner
			if claims.UserType != "partner" {
				log.Printf("❌ AUTH: Non-partner user attempting to use partner agent endpoint - BLOCKING REQUEST")
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"success": false,
					"error":   "Forbidden",
					"message": "Only partner accounts can access this endpoint.",
				})
				return
			}

			// Check partner_profiles.status - block access if status is false
			// Uses Redis cache (60s TTL) to reduce database queries
			ctx := r.Context()
			partnerEmail := claims.Email // Use email from JWT claims
			log.Printf("🔍 AUTH: Checking partner profile status for email: %s", partnerEmail)
			
			// Check Redis cache first (60s TTL to reduce database queries)
			cacheKey := fmt.Sprintf("partner:status:%s", partnerEmail)
			cachedStatus, err := redisClient.Get(ctx, cacheKey)
			
			var isAuthorized bool
			var shopName string
			cacheHit := false
			
			if err == nil {
				// Cache hit - parse cached status
				statusBool, parseErr := strconv.ParseBool(cachedStatus)
				if parseErr == nil {
					isAuthorized = statusBool
					cacheHit = true
					log.Printf("✅ AUTH: Partner status retrieved from cache - Email: %s, Status: %v", partnerEmail, isAuthorized)
					
					// If status is false, block immediately
					if !isAuthorized {
						log.Printf("❌ AUTH: Partner account not authorized (cached) - Email: %s, Status: false - BLOCKING ACCESS", partnerEmail)
						w.Header().Set("Content-Type", "application/json")
						w.WriteHeader(http.StatusUnauthorized)
						json.NewEncoder(w).Encode(map[string]interface{}{
							"success": false,
							"error":   "Unauthorized",
							"message": "Your partner account is not authorized. Please contact administrator.",
						})
						return
					}
					
					// Status is true, proceed (we don't cache shop name, but that's okay for this check)
					log.Printf("✅ AUTH: Partner profile verified (cached) - Status: true")
				} else {
					// Cached value is invalid, fall through to database query
					log.Printf("⚠️ AUTH: Invalid cached status value, querying database")
					cacheHit = false
				}
			} else if err != redis.Nil {
				// Redis error (not a cache miss) - log but continue to database query
				log.Printf("⚠️ AUTH: Redis cache error (non-fatal): %v, querying database", err)
				cacheHit = false
			}
			
			// Cache miss or invalid cache value - query database
			if !cacheHit {
				partnerProfile, dbErr := partnerProfileRepository.GetByAccountEmail(ctx, partnerEmail)
				if dbErr != nil {
					log.Printf("❌ AUTH: Partner profile not found for email: %s - Error: %v", partnerEmail, dbErr)
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
				shopName = partnerProfile.ShopName
				
				// Cache the status for 60 seconds
				statusStr := strconv.FormatBool(isAuthorized)
				if cacheErr := redisClient.Set(ctx, cacheKey, statusStr, 60*time.Second); cacheErr != nil {
					log.Printf("⚠️ AUTH: Failed to cache partner status (non-fatal): %v", cacheErr)
				} else {
					log.Printf("✅ AUTH: Partner status cached for 60 seconds - Email: %s, Status: %v", partnerEmail, isAuthorized)
				}
				
				// Check if partner status is true (authorized)
				if !isAuthorized {
					log.Printf("❌ AUTH: Partner account not authorized - Email: %s, Status: false - BLOCKING ACCESS", partnerEmail)
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusUnauthorized)
					json.NewEncoder(w).Encode(map[string]interface{}{
						"success": false,
						"error":   "Unauthorized",
						"message": "Your partner account is not authorized. Please contact administrator.",
					})
					return
				}
				
				log.Printf("✅ AUTH: Partner profile verified - Status: true, Shop: %s", shopName)
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
			ctx = context.WithValue(ctx, "user", user)
			r = r.WithContext(ctx)

			// Call the next handler
			next(w, r)
		}
	}
}
