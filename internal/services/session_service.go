package services

import (
	"context"
	"encoding/json"
	"fmt"
	"print-pro-backend/internal/infrastructure"
	"print-pro-backend/internal/models"
	"time"
)

// SessionService handles session management with Redis
type SessionService struct {
	redisClient *infrastructure.RedisClient
	sessionTTL  time.Duration
}

// NewSessionService creates a new session service
func NewSessionService(redisClient *infrastructure.RedisClient) *SessionService {
	return &SessionService{
		redisClient: redisClient,
		sessionTTL:  7 * 24 * time.Hour, // 7 days
	}
}

// CreateSession creates a new session in Redis
func (s *SessionService) CreateSession(ctx context.Context, token string, user *models.User) error {
	sessionKey := fmt.Sprintf("session:%s", token)
	
	sessionData := map[string]interface{}{
		"user_id":    user.ID,
		"email":      user.Email,
		"name":       user.Name,
		"created_at": time.Now().Unix(),
	}
	
	data, err := json.Marshal(sessionData)
	if err != nil {
		return fmt.Errorf("failed to marshal session data: %w", err)
	}
	
	return s.redisClient.Set(ctx, sessionKey, data, s.sessionTTL)
}

// GetSession retrieves session data from Redis
func (s *SessionService) GetSession(ctx context.Context, token string) (*models.User, error) {
	sessionKey := fmt.Sprintf("session:%s", token)
	
	data, err := s.redisClient.Get(ctx, sessionKey)
	if err != nil {
		return nil, fmt.Errorf("session not found or expired")
	}
	
	var sessionData map[string]interface{}
	if err := json.Unmarshal([]byte(data), &sessionData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal session data: %w", err)
	}
	
	user := &models.User{
		ID:    fmt.Sprintf("%v", sessionData["user_id"]),
		Email: fmt.Sprintf("%v", sessionData["email"]),
		Name:  fmt.Sprintf("%v", sessionData["name"]),
	}
	
	return user, nil
}

// DeleteSession removes a session from Redis
func (s *SessionService) DeleteSession(ctx context.Context, token string) error {
	sessionKey := fmt.Sprintf("session:%s", token)
	return s.redisClient.Delete(ctx, sessionKey)
}

// RefreshSession extends the session TTL
func (s *SessionService) RefreshSession(ctx context.Context, token string) error {
	sessionKey := fmt.Sprintf("session:%s", token)
	
	// Check if session exists
	exists, err := s.redisClient.Exists(ctx, sessionKey)
	if err != nil || !exists {
		return fmt.Errorf("session not found")
	}
	
	// Get current session data
	data, err := s.redisClient.Get(ctx, sessionKey)
	if err != nil {
		return err
	}
	
	// Reset with new TTL
	return s.redisClient.Set(ctx, sessionKey, data, s.sessionTTL)
}

