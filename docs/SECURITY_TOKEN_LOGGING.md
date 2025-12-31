# Security: Token Logging Policy

## ⚠️ CRITICAL: NEVER LOG TOKENS

**Access tokens and refresh tokens are sensitive credentials. They must NEVER be logged, printed, or included in error messages.**

## What is Safe to Log

✅ **SAFE:**
- User IDs (e.g., `log.Printf("User: %s", userID)`)
- Status messages (e.g., `log.Printf("Token validated successfully")`)
- Error types (e.g., `log.Printf("Token validation failed: %v", err)`)
- Cookie names (e.g., `log.Printf("Refresh token cookie not found")`)

## What is NOT Safe to Log

❌ **NEVER LOG:**
- Access token values
- Refresh token values
- JWT token strings
- Cookie values containing tokens
- Any variable that contains a token

## Code Examples

### ❌ WRONG - DO NOT DO THIS:

```go
// NEVER log the actual token
log.Printf("Access token: %s", accessToken)
log.Printf("Refresh token: %s", refreshToken)
log.Printf("Token received: %v", token)
fmt.Println("Token:", token)
```

### ✅ CORRECT - DO THIS INSTEAD:

```go
// Log status messages only
log.Printf("✅ Access token generated successfully for user: %s", userID)
log.Printf("🔍 Validating access token...")
log.Printf("❌ Token validation failed - %v", err) // Error type, not token value

// If you need to debug, use the masking utility
import "print-pro-backend/internal/utils"
log.Printf("Token (masked): %s", utils.MaskToken(token))
```

## Security Comments in Code

All token-handling code includes security comments:

```go
refreshToken := cookie.Value
// SECURITY NOTE: refreshToken variable contains sensitive data - never log it

newAccessToken, err := h.jwtService.GenerateAccessToken(...)
// SECURITY NOTE: newAccessToken contains sensitive data - never log it, only return in response
```

## Token Masking Utility

If you ever need to log tokens for debugging (e.g., in development), use the masking utility:

```go
import "print-pro-backend/internal/utils"

token := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
masked := utils.MaskToken(token)
// Result: "eyJh...J9..." (first 4 + last 4 characters)

// Or shorter version:
masked := utils.MaskTokenShort(token)
// Result: "eyJhbGciOiJ..." (first 10 characters only)
```

## Error Messages

JWT validation errors are safe - they don't include token values:
- `"invalid token"`
- `"token expired"`
- `"token is not an access token"`
- `"unexpected signing method"`

These error messages are safe to log.

## Review Checklist

Before committing code that handles tokens:

- [ ] No `log.Printf` statements with token variables
- [ ] No `fmt.Printf` statements with token variables
- [ ] No token values in error messages sent to clients
- [ ] Security comments added near token variables
- [ ] Only status messages and user IDs are logged

## Reporting Security Issues

If you find any code that logs tokens, immediately:
1. Remove the logging statement
2. Add security comments
3. Notify the team
4. Review git history for any tokens that may have been logged

---

**Remember: Tokens are like passwords - treat them with the same level of security!**

