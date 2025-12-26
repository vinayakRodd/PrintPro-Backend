package services

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"print-pro-backend/internal/config"
	"print-pro-backend/internal/models"
	"time"
)

// GoogleAuthService handles Google authentication
type GoogleAuthService struct {
	config *config.Config
}

// NewGoogleAuthService creates a new GoogleAuthService instance
func NewGoogleAuthService(cfg *config.Config) *GoogleAuthService {
	return &GoogleAuthService{
		config: cfg,
	}
}

// VerifyGoogleToken verifies the Google ID token and returns user information
func (s *GoogleAuthService) VerifyGoogleToken(ctx context.Context, idToken string) (*models.User, error) {
	// Verify token with Google
	url := fmt.Sprintf("https://oauth2.googleapis.com/tokeninfo?id_token=%s", idToken)
	
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to verify token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("token verification failed: %s", string(body))
	}

	var tokenInfo struct {
		Iss           string `json:"iss"`
		Aud           string `json:"aud"`
		Sub           string `json:"sub"`
		Email         string `json:"email"`
		EmailVerified string `json:"email_verified"`
		Name          string `json:"name"`
		Picture       string `json:"picture"`
		GivenName     string `json:"given_name"`
		FamilyName    string `json:"family_name"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&tokenInfo); err != nil {
		return nil, fmt.Errorf("failed to decode token info: %w", err)
	}

	// Verify the audience (client ID) matches
	if s.config.GoogleClientID != "" && tokenInfo.Aud != s.config.GoogleClientID {
		return nil, fmt.Errorf("invalid token audience")
	}

	// Verify issuer
	if tokenInfo.Iss != "https://accounts.google.com" && tokenInfo.Iss != "accounts.google.com" {
		return nil, fmt.Errorf("invalid token issuer")
	}

	// Check if email is verified
	if tokenInfo.EmailVerified != "true" {
		return nil, fmt.Errorf("email not verified")
	}

	// Create user object
	user := &models.User{
		ID:       tokenInfo.Sub, // Google user ID
		Email:    tokenInfo.Email,
		Name:     tokenInfo.Name,
		Picture:  tokenInfo.Picture,
		Provider: "google",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	return user, nil
}

// RegisterOrLoginUser registers a new user or returns existing user
// In a real application, this would interact with a database
func (s *GoogleAuthService) RegisterOrLoginUser(ctx context.Context, user *models.User) (*models.User, error) {
	// TODO: Implement database logic here
	// For now, we'll just return the user as if they were registered
	// In production, you would:
	// 1. Check if user exists by email or provider ID
	// 2. If exists, update last login time
	// 3. If not exists, create new user record
	// 4. Return the user with database ID
	
	return user, nil
}

// GenerateSessionToken generates a session token for the user
// In production, use JWT or similar
func (s *GoogleAuthService) GenerateSessionToken(userID string) (string, error) {
	// Simple token generation (in production, use JWT)
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

