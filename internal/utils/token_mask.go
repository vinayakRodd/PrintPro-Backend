package utils

import (
	"strings"
)

// MaskToken masks a token for safe logging
// Returns first 4 and last 4 characters with asterisks in between
// Example: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..." -> "eyJh...J9..."
func MaskToken(token string) string {
	if token == "" {
		return "[EMPTY]"
	}

	// If token is too short, just return asterisks
	if len(token) <= 8 {
		return "****"
	}

	// Show first 4 and last 4 characters
	first := token[:4]
	last := token[len(token)-4:]
	masked := strings.Repeat("*", len(token)-8)

	return first + masked + last
}

// MaskTokenShort masks a token showing only first few characters
// Useful for very long tokens
func MaskTokenShort(token string) string {
	if token == "" {
		return "[EMPTY]"
	}

	if len(token) <= 10 {
		return "****"
	}

	// Show only first 10 characters
	return token[:10] + "..."
}

