package shared

import (
	"context"
	"net/http"
)

// TokenHelper interface for token generation
type TokenHelper interface {
	GenerateTokensAndSetCookies(w http.ResponseWriter, userID, email, userType string, ctx context.Context) (string, string, error)
}

