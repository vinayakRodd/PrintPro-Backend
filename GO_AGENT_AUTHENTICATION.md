# Go Agent Authentication - Backend Changes Summary

## Overview

The backend has been updated to support JWT-based authentication for the Go printer agent while maintaining backward compatibility with the Python agent (which doesn't use authentication).

## Changes Implemented

### 1. ✅ Partner Login Returns Refresh Token

**File:** `internal/api/handlers/auth_handler/partner/login_handler.go`

**Change:** Login response now includes both `access_token` and `refresh_token` in the JSON body.

**Before:**
```go
response := models.GoogleSignInResponse{
    Success: true,
    Message: "Partner login successful",
    User:    authUser,
    Token:   accessToken, // Only access token
}
```

**After:**
```go
response := map[string]interface{}{
    "success":       true,
    "message":      "Partner login successful",
    "user":         authUser,
    "access_token": accessToken,   // Access token
    "token":        accessToken,    // Alias for backward compatibility
    "refresh_token": refreshToken, // Refresh token in body (for Go agent)
}
```

**Why:** Go agent needs refresh token in response body (can't easily read cookies).

---

### 2. ✅ Refresh Token Endpoint Accepts Authorization Header

**File:** `internal/api/handlers/auth_handler/session_handlers/refresh_token_handler.go`

**Change:** Refresh endpoint now accepts refresh token from either:
- `Authorization: Bearer <refresh_token>` header (for Go agent)
- `refresh_token` cookie (for web frontend)

**Before:**
```go
cookie, err := r.Cookie("refresh_token")
if err != nil || cookie.Value == "" {
    // Error - only cookie supported
}
```

**After:**
```go
// Try Authorization header first (for Go agent)
authHeader := r.Header.Get("Authorization")
if authHeader != "" {
    parts := strings.Split(authHeader, " ")
    if len(parts) == 2 && parts[0] == "Bearer" {
        refreshToken = parts[1]
    }
}

// Fallback to cookie (for web frontend)
if refreshToken == "" {
    cookie, err := r.Cookie("refresh_token")
    if err == nil && cookie.Value != "" {
        refreshToken = cookie.Value
    }
}
```

**Why:** Go agent can't easily manage cookies, needs header-based authentication.

---

### 3. ✅ Optional JWT Authentication Middleware

**File:** `internal/middleware/auth_middleware/optional_auth.go` (NEW)

**Purpose:** Validates JWT if present, but doesn't fail if missing (backward compatible).

**How it works:**
1. Checks for `Authorization: Bearer <token>` header
2. If present: Validates JWT and attaches user to context
3. If missing: Proceeds without authentication (Python agent compatibility)
4. If invalid: Logs warning but proceeds (graceful degradation)

**Key Features:**
- ✅ Backward compatible (Python agent works without changes)
- ✅ Validates JWT when provided (Go agent)
- ✅ Verifies user is a partner
- ✅ Attaches user to context for logging/auditing

---

### 4. ✅ Partner Agent Endpoints Use Optional Auth

**File:** `internal/app/app.go`

**Change:** All partner agent endpoints now use `OptionalAuthMiddleware`.

**Endpoints Updated:**
- `/api/partner-agent/fetch-job`
- `/api/partner-agent/confirm-print`
- `/api/partner-agent/confirm`
- `/api/partner-agent/sync-printers`
- `/api/partner-agent/reprint`

**Before:**
```go
http.HandleFunc("/api/partner-agent/fetch-job", cors.CORS(agentHandler.FetchJob))
```

**After:**
```go
optionalAuth := auth_middleware.OptionalAuthMiddleware(a.services.JWTService)
http.HandleFunc("/api/partner-agent/fetch-job", cors.CORS(optionalAuth(agentHandler.FetchJob)))
```

---

## Authentication Flow

### Go Agent Flow:

1. **Login:**
   ```
   POST /api/auth/login/partner
   Body: { "email": "...", "password": "..." }
   Response: {
     "access_token": "...",
     "refresh_token": "...",
     ...
   }
   ```

2. **Store Tokens:**
   - Go agent saves both tokens in encrypted file
   - Access token expires in 15 minutes
   - Refresh token expires in 7 days

3. **Make Requests:**
   ```
   GET /api/partner-agent/fetch-job
   Header: Authorization: Bearer <access_token>
   ```

4. **Auto-Refresh (on 401):**
   ```
   POST /api/auth/refresh
   Header: Authorization: Bearer <refresh_token>
   Response: { "access_token": "..." }
   ```

5. **Re-login (if refresh expired):**
   - If refresh token expired, Go agent prompts user to login again

### Python Agent Flow (Unchanged):

1. **No Authentication:**
   - Python agent continues to work without JWT
   - Requests work without `Authorization` header
   - Backward compatible ✅

---

## Security Benefits

1. **Go Agent:**
   - ✅ Authenticated requests (JWT validation)
   - ✅ User identification for logging
   - ✅ Token-based security

2. **Backward Compatibility:**
   - ✅ Python agent still works (no breaking changes)
   - ✅ Both agents can run simultaneously
   - ✅ Graceful degradation if JWT invalid

3. **Audit Trail:**
   - ✅ Authenticated requests logged with user ID
   - ✅ Unauthenticated requests logged as "backward compatibility mode"

---

## Testing

### Test Go Agent:
1. Login with partner credentials
2. Verify tokens received in response
3. Make authenticated request to `/api/partner-agent/fetch-job`
4. Verify request succeeds with JWT

### Test Python Agent:
1. Run Python agent (no changes needed)
2. Verify it still works without authentication
3. Verify requests succeed without JWT

### Test Both Simultaneously:
1. Run both agents
2. Verify both can fetch jobs
3. Verify Go agent requests are logged with user ID

---

## Notes

- **No OTP Required:** Partner login doesn't require OTP (unlike customer login)
- **Token Storage:** Go agent stores tokens encrypted in `~/.print_agent/tokens.enc`
- **Refresh Logic:** Go agent automatically refreshes access token on 401 errors
- **Cookie Support:** Refresh endpoint still supports cookies for web frontend

---

## Files Modified

1. ✅ `internal/api/handlers/auth_handler/partner/login_handler.go`
2. ✅ `internal/api/handlers/auth_handler/session_handlers/refresh_token_handler.go`
3. ✅ `internal/middleware/auth_middleware/optional_auth.go` (NEW)
4. ✅ `internal/app/app.go`
5. ✅ `partner_agent_go.go` (updated to use Authorization header for refresh)

---

## Next Steps

1. **Test the changes:**
   ```bash
   go build ./...
   ```

2. **Run the server** and test with Go agent

3. **Verify backward compatibility** with Python agent

4. **Monitor logs** to see authentication status

---

## Summary

✅ **Go agent can now authenticate with JWT**  
✅ **Python agent continues to work without changes**  
✅ **Both agents can run simultaneously**  
✅ **Backward compatible - no breaking changes**
