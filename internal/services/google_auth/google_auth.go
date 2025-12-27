package google_auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"print-pro-backend/internal/config"
	"print-pro-backend/internal/models"
	"print-pro-backend/internal/repositories"
	"time"
)

// GoogleAuthService handles Google authentication
type GoogleAuthService struct {
	config         *config.Config
	userRepository *repositories.UserRepository
}

// NewGoogleAuthService creates a new GoogleAuthService instance
func NewGoogleAuthService(cfg *config.Config, userRepo *repositories.UserRepository) *GoogleAuthService {
	return &GoogleAuthService{
		config:         cfg,
		userRepository: userRepo,
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

	body, _ := io.ReadAll(resp.Body)
	
	if resp.StatusCode != http.StatusOK {
		// Try to parse error from Google
		var googleError struct {
			Error            string `json:"error"`
			ErrorDescription string `json:"error_description"`
		}
		if err := json.Unmarshal(body, &googleError); err == nil {
			return nil, fmt.Errorf("token verification failed: %s - %s", googleError.Error, googleError.ErrorDescription)
		}
		
		return nil, fmt.Errorf("token verification failed")
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
		Exp           string `json:"exp"` // Token expiration time (Unix timestamp)
		Iat           string `json:"iat"` // Token issued at time (Unix timestamp)
	}

	if err := json.Unmarshal(body, &tokenInfo); err != nil {
		return nil, fmt.Errorf("failed to decode token info: %w", err)
	}

	// Verify the audience (client ID) matches - REQUIRED for security
	// This ensures tokens are only accepted from our specific application
	if tokenInfo.Aud != s.config.GoogleClientID {
		return nil, fmt.Errorf("invalid token audience: client ID mismatch")
	}
	log.Printf("Verified")

	// Verify issuer
	if tokenInfo.Iss != "https://accounts.google.com" && tokenInfo.Iss != "accounts.google.com" {
		return nil, fmt.Errorf("invalid token issuer")
	}

	// Check if email is verified
	if tokenInfo.EmailVerified != "true" {
		return nil, fmt.Errorf("email not verified")
	}

	// Verify token expiration (security check)
	if tokenInfo.Exp != "" {
		expTime, err := parseUnixTimestamp(tokenInfo.Exp)
		if err == nil {
			if time.Now().After(expTime) {
				return nil, fmt.Errorf("token has expired")
			}
		}
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
// Checks if user exists by email, creates new user if not found
func (s *GoogleAuthService) RegisterOrLoginUser(ctx context.Context, googleUser *models.User) (*models.User, error) {
	// Check if user already exists by email
	dbUser, err := s.userRepository.GetByEmail(ctx, googleUser.Email)
	if err == nil {
		// User exists - return existing user
		// Convert database user to auth user model
		return &models.User{
			ID:        fmt.Sprintf("%d", dbUser.ID),
			Email:     dbUser.Email,
			Name:      dbUser.FullName,
			Provider:  "google",
			CreatedAt: dbUser.CreatedAt,
			UpdatedAt: time.Now(),
		}, nil
	}

	// User doesn't exist - create new user
	// For Google OAuth users, we use a placeholder for password_hash since they authenticate via Google
	// The password_hash field is NOT NULL, so we store "oauth_google" as a marker
	// In production, you might want to add a provider field to track OAuth users separately
	newUser, err := s.userRepository.Create(ctx, googleUser.Name, googleUser.Email, "oauth_google")
	if err != nil {
		return nil, fmt.Errorf("failed to register user: %w", err)
	}

	log.Printf("New user registered (ID: %d)", newUser.ID)

	// Convert database user to auth user model
	return &models.User{
		ID:        fmt.Sprintf("%d", newUser.ID),
		Email:     newUser.Email,
		Name:      newUser.FullName,
		Provider:  "google",
		CreatedAt: newUser.CreatedAt,
		UpdatedAt: time.Now(),
	}, nil
}

// GenerateSessionToken generates a session token for the user
// Uses user ID (database ID) for session token generation
func (s *GoogleAuthService) GenerateSessionToken(userID string) (string, error) {
	// Simple token generation (in production, use JWT)
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// parseUnixTimestamp parses a Unix timestamp string to time.Time
func parseUnixTimestamp(ts string) (time.Time, error) {
	var timestamp int64
	if _, err := fmt.Sscanf(ts, "%d", &timestamp); err != nil {
		return time.Time{}, err
	}
	return time.Unix(timestamp, 0), nil
}

