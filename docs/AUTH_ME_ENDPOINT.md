# Authentication & `/api/auth/me` Endpoint

## Overview

The `/api/auth/me` endpoint allows you to check if a user is authenticated and get their current user information. This is perfect for checking authentication status when the page refreshes.

## How It Works

1. **User signs in** → Session token stored in Redis + Cookie set in browser
2. **User refreshes page** → Frontend calls `/api/auth/me` with cookie
3. **Backend verifies** → Checks cookie → Validates session in Redis → Returns user

## Endpoint

### `GET /api/auth/me`

**Description:** Returns the current authenticated user's information

**Authentication:** Required (session cookie)

**Headers:**
- Cookie: `session_token={token}` (automatically sent by browser)

**Success Response (200 OK):**
```json
{
  "success": true,
  "message": "User authenticated",
  "user": {
    "id": "google-user-id",
    "email": "user@example.com",
    "name": "John Doe",
    "picture": "https://...",
    "provider": "google",
    "created_at": "2025-12-26T10:00:00Z",
    "updated_at": "2025-12-26T10:00:00Z"
  },
  "token": ""
}
```

**Error Response (401 Unauthorized):**
```json
{
  "success": false,
  "message": "Unauthorized",
  "error": "No session token found"
}
```
or
```json
{
  "success": false,
  "message": "Unauthorized",
  "error": "Invalid or expired session"
}
```

## Frontend Usage

### React/Next.js Example

```javascript
// Check if user is authenticated on page load/refresh
async function checkAuth() {
  try {
    const response = await fetch('http://localhost:8080/api/auth/me', {
      method: 'GET',
      credentials: 'include', // Important: sends cookies
      headers: {
        'Content-Type': 'application/json',
      },
    });

    const data = await response.json();

    if (data.success && data.user) {
      // User is authenticated
      console.log('User:', data.user);
      setUser(data.user);
      setIsAuthenticated(true);
    } else {
      // User is not authenticated
      setIsAuthenticated(false);
      setUser(null);
    }
  } catch (error) {
    // Network error or not authenticated
    setIsAuthenticated(false);
    setUser(null);
  }
}

// Call on component mount or page refresh
useEffect(() => {
  checkAuth();
}, []);
```

### Axios Example

```javascript
import axios from 'axios';

// Configure axios to send cookies
axios.defaults.withCredentials = true;

async function checkAuth() {
  try {
    const response = await axios.get('http://localhost:8080/api/auth/me');
    
    if (response.data.success) {
      setUser(response.data.user);
      setIsAuthenticated(true);
    }
  } catch (error) {
    if (error.response?.status === 401) {
      // Not authenticated
      setIsAuthenticated(false);
      setUser(null);
    }
  }
}
```

## Authentication Flow

```
┌─────────────┐
│   Frontend  │
└──────┬──────┘
       │
       │ 1. GET /api/auth/me (with cookie)
       ▼
┌─────────────┐
│   Backend   │
└──────┬──────┘
       │
       │ 2. Extract session_token from cookie
       ▼
┌─────────────┐
│Auth Middleware│
└──────┬──────┘
       │
       │ 3. Check session in Redis
       ▼
┌─────────────┐
│    Redis    │
└──────┬──────┘
       │
       │ 4. Session found? Return user
       │    Session not found? Return 401
       ▼
┌─────────────┐
│   Frontend  │
└─────────────┘
```

## Protecting Other Routes

To protect any route, wrap it with the auth middleware:

```go
// In main.go
http.HandleFunc("/api/protected-route", 
  corsHandler(
    rateLimiter.LimitMiddleware(
      authMiddlewareFunc(protectedHandler)
    )
  )
)
```

## Session Verification Process

1. **Cookie Extraction**: Gets `session_token` from request cookie
2. **Redis Lookup**: Checks if session exists in Redis with key `session:{token}`
3. **User Retrieval**: If found, retrieves user data from Redis
4. **Context Injection**: Adds user to request context for handler use
5. **Authorization**: If session invalid/missing, returns 401

## Error Handling

- **No Cookie**: Returns 401 with "No session token found"
- **Invalid Token**: Returns 401 with "Invalid or expired session"
- **Redis Down**: Will fail (consider adding fallback logic)

## Best Practices

1. **Call on App Load**: Check auth status when app starts
2. **Handle 401**: Redirect to login if not authenticated
3. **Refresh Token**: Optionally refresh session TTL on each `/me` call
4. **Error Handling**: Handle network errors gracefully

