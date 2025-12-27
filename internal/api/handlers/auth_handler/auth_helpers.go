package auth_handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
	"print-pro-backend/internal/models"
	userModel "print-pro-backend/internal/models/user"
)

// sendJSONResponse sends a JSON response
func (h *AuthHandler) sendJSONResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	
	// Encode JSON first to check for errors before writing header
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	if err := encoder.Encode(data); err != nil {
		log.Printf("ERROR: Failed to encode JSON response - Status: %d, Error: %v", statusCode, err)
		// If encoding fails, send error response
		w.WriteHeader(http.StatusInternalServerError)
		errorResponse := models.ErrorResponse{
			Success: false,
			Message: "Failed to encode response",
			Error:   err.Error(),
		}
		json.NewEncoder(w).Encode(errorResponse)
		return
	}
	
	// If encoding succeeded, write header and send response
	w.WriteHeader(statusCode)
	if _, err := w.Write(buf.Bytes()); err != nil {
		log.Printf("ERROR: Failed to write JSON response - Status: %d, Error: %v", statusCode, err)
	} else {
		log.Printf("JSON response sent successfully - Status: %d", statusCode)
	}
}

// sendErrorResponse sends an error response
func (h *AuthHandler) sendErrorResponse(w http.ResponseWriter, statusCode int, message, errorDetail string) {
	response := models.ErrorResponse{
		Success: false,
		Message: message,
		Error:   errorDetail,
	}
	h.sendJSONResponse(w, statusCode, response)
}

// getUserFromCacheOrDB gets user from Redis cache first, then falls back to database
// Stores user in cache for future lookups (for scalability)
func (h *AuthHandler) getUserFromCacheOrDB(ctx context.Context, email string) (*userModel.User, error) {
	// Normalize email (lowercase, trim)
	email = strings.ToLower(strings.TrimSpace(email))
	
	// Check Redis cache first
	cacheKey := fmt.Sprintf("user:email:%s", email)
	cachedData, err := h.redisClient.Get(ctx, cacheKey)
	if err == nil && cachedData != "" {
		// User found in cache - parse JSON
		var dbUser userModel.User
		if err := json.Unmarshal([]byte(cachedData), &dbUser); err == nil {
			log.Printf("User found in cache")
			return &dbUser, nil
		}
	}
	
	// Not in cache - check database
	log.Printf("User not in cache, checking database")
	dbUser, err := h.userRepository.GetByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	
	// Store in cache for future lookups (1 hour TTL)
	userData, err := json.Marshal(dbUser)
	if err == nil {
		h.redisClient.Set(ctx, cacheKey, userData, time.Hour)
		log.Printf("User stored in cache")
	}
	
	return dbUser, nil
}

