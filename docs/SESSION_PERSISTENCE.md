# Session Persistence on Page Refresh

## Overview

When users refresh the page in the Partner or Student dashboard, they should remain logged in. This document explains how to implement session persistence using the authentication APIs.

## Problem

On page refresh, the frontend loses its authentication state and needs to verify if the user is still authenticated using the backend APIs.

## Solution

Use the following API endpoints to check and maintain authentication:

1. **`GET /api/auth/me`** - Check if user is authenticated
2. **`POST /api/auth/refresh`** - Refresh access token if expired

---

## API Endpoints

### 1. Get Current User (`GET /api/auth/me`)

**Description:** Returns the current authenticated user's information. Use this on every page load/refresh to check if the user is still authenticated.

**Endpoint:** `GET /api/auth/me`

**Authentication:** Required (access token in cookie or Authorization header)

**Headers:**
- Cookie: `access_token={token}` (automatically sent by browser)
- OR Authorization: `Bearer {token}` (if using header)

**Success Response (200 OK):**
```json
{
  "success": true,
  "message": "User authenticated",
  "user": {
    "id": "user-id",
    "email": "user@example.com",
    "name": "User Name",
    "picture": "https://...",
    "provider": "google",
    "user_type": "partner",  // or "customer"
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
  "error": "User not found in session"
}
```

**Usage:**
- Call this on every page load/refresh
- If successful, user is authenticated - set user state
- If fails (401), try refresh token endpoint

---

### 2. Refresh Access Token (`POST /api/auth/refresh`)

**Description:** Gets a new access token using the refresh token stored in cookies. Use this when `/api/auth/me` returns 401.

**Endpoint:** `POST /api/auth/refresh`

**Authentication:** Requires `refresh_token` cookie (automatically sent by browser)

**Headers:**
- Cookie: `refresh_token={token}` (automatically sent by browser)
- Content-Type: `application/json`

**Success Response (200 OK):**
```json
{
  "success": true,
  "access_token": "new-access-token-jwt",
  "message": "Access token refreshed successfully"
}
```

**Error Response (401 Unauthorized):**
```json
{
  "success": false,
  "message": "Unauthorized",
  "error": "Invalid or expired refresh token"
}
```

**Usage:**
- Call this when `/api/auth/me` returns 401
- If successful, you get a new access token
- Then call `/api/auth/me` again to get user info
- If fails, redirect to login page

---

## Frontend Implementation

### React/Next.js Example

```javascript
// utils/auth.js or hooks/useAuth.js

/**
 * Check if user is authenticated by calling /api/auth/me
 */
export async function checkAuth() {
  try {
    const response = await fetch('http://localhost:8080/api/auth/me', {
      method: 'GET',
      credentials: 'include', // IMPORTANT: Include cookies
      headers: {
        'Content-Type': 'application/json',
      }
    });

    if (response.ok) {
      const data = await response.json();
      // User is authenticated
      return {
        success: true,
        user: data.user
      };
    } else {
      // User not authenticated, try refresh token
      return {
        success: false,
        needsRefresh: true
      };
    }
  } catch (error) {
    console.error('Auth check failed:', error);
    return {
      success: false,
      needsRefresh: true,
      error: error.message
    };
  }
}

/**
 * Refresh access token using refresh token cookie
 */
export async function refreshToken() {
  try {
    const response = await fetch('http://localhost:8080/api/auth/refresh', {
      method: 'POST',
      credentials: 'include', // IMPORTANT: Include cookies
      headers: {
        'Content-Type': 'application/json',
      }
    });

    if (response.ok) {
      const data = await response.json();
      // New access token received
      return {
        success: true,
        accessToken: data.access_token
      };
    } else {
      // Refresh token expired or invalid
      return {
        success: false,
        error: 'Refresh token expired'
      };
    }
  } catch (error) {
    console.error('Token refresh failed:', error);
    return {
      success: false,
      error: error.message
    };
  }
}

/**
 * Complete authentication flow for page refresh
 * Returns user object if authenticated, null if not
 */
export async function initializeAuth() {
  // Step 1: Try to get current user
  const authCheck = await checkAuth();
  
  if (authCheck.success) {
    // User is authenticated
    return authCheck.user;
  }
  
  // Step 2: If auth check failed, try refreshing token
  if (authCheck.needsRefresh) {
    const refreshResult = await refreshToken();
    
    if (refreshResult.success) {
      // Token refreshed, try getting user again
      const retryAuth = await checkAuth();
      if (retryAuth.success) {
        return retryAuth.user;
      }
    }
  }
  
  // Not authenticated - return null
  return null;
}
```

### Usage in Dashboard Components

```javascript
// components/PartnerDashboard.jsx or pages/partner/dashboard.jsx

import { useState, useEffect } from 'react';
import { useRouter } from 'next/router';
import { initializeAuth } from '../utils/auth';

export default function PartnerDashboard() {
  const [user, setUser] = useState(null);
  const [loading, setLoading] = useState(true);
  const router = useRouter();

  useEffect(() => {
    async function checkAuthentication() {
      const authenticatedUser = await initializeAuth();
      
      if (authenticatedUser) {
        // User is authenticated
        setUser(authenticatedUser);
        setLoading(false);
      } else {
        // Not authenticated, redirect to login
        router.push('/login');
      }
    }

    checkAuthentication();
  }, []); // Run on mount and page refresh

  if (loading) {
    return <div>Loading...</div>;
  }

  if (!user) {
    return null; // Will redirect
  }

  return (
    <div>
      <h1>Partner Dashboard</h1>
      <p>Welcome, {user.name}!</p>
      <p>Email: {user.email}</p>
      {/* Rest of your dashboard */}
    </div>
  );
}
```

### Usage in Student/Customer Dashboard

```javascript
// components/CustomerDashboard.jsx or pages/customer/dashboard.jsx

import { useState, useEffect } from 'react';
import { useRouter } from 'next/router';
import { initializeAuth } from '../utils/auth';

export default function CustomerDashboard() {
  const [user, setUser] = useState(null);
  const [loading, setLoading] = useState(true);
  const router = useRouter();

  useEffect(() => {
    async function checkAuthentication() {
      const authenticatedUser = await initializeAuth();
      
      if (authenticatedUser) {
        // Verify user is a customer
        if (authenticatedUser.user_type !== 'customer') {
          router.push('/login');
          return;
        }
        
        setUser(authenticatedUser);
        setLoading(false);
      } else {
        router.push('/login');
      }
    }

    checkAuthentication();
  }, []);

  if (loading) {
    return <div>Loading...</div>;
  }

  if (!user) {
    return null;
  }

  return (
    <div>
      <h1>Student Dashboard</h1>
      <p>Welcome, {user.name}!</p>
      {/* Rest of your dashboard */}
    </div>
  );
}
```

---

## Important Notes

### 1. Always Include Cookies

**CRITICAL:** Always use `credentials: 'include'` in fetch requests to send cookies:

```javascript
fetch('http://localhost:8080/api/auth/me', {
  credentials: 'include', // This sends cookies automatically
  // ...
})
```

### 2. Cookie Configuration

The backend sets cookies with these names:
- `access_token` - Short-lived access token (15 minutes)
- `refresh_token` - Long-lived refresh token (7 days)

Both cookies are:
- HttpOnly (not accessible via JavaScript)
- Secure (HTTPS only in production)
- SameSite (CSRF protection)

### 3. Flow Diagram

```
Page Refresh
    ↓
Call GET /api/auth/me
    ↓
Is user authenticated?
    ├─ YES → Set user state, show dashboard
    └─ NO → Call POST /api/auth/refresh
                ↓
            Token refreshed?
                ├─ YES → Call GET /api/auth/me again → Set user state
                └─ NO → Redirect to /login
```

### 4. Error Handling

Always handle these scenarios:

1. **Network errors** - Show error message, allow retry
2. **401 Unauthorized** - Try refresh, then redirect to login
3. **403 Forbidden** - User doesn't have access, redirect appropriately
4. **500 Server Error** - Show error message, log for debugging

### 5. User Type Validation

After getting user from `/api/auth/me`, verify the user type matches the dashboard:

```javascript
if (user.user_type !== 'partner') {
  // Redirect to customer dashboard or show error
  router.push('/customer/dashboard');
}
```

---

## Testing

### Test Cases

1. **Valid Session**
   - User logs in
   - Refresh page
   - Should remain logged in
   - `/api/auth/me` should return user

2. **Expired Access Token, Valid Refresh Token**
   - User logs in
   - Wait for access token to expire (15 minutes)
   - Refresh page
   - Should automatically refresh token
   - Should remain logged in

3. **Expired Refresh Token**
   - User logs in
   - Wait for refresh token to expire (7 days)
   - Refresh page
   - Should redirect to login

4. **No Session**
   - User not logged in
   - Try to access dashboard
   - Should redirect to login

---

## Troubleshooting

### Issue: User gets logged out on every refresh

**Solution:**
- Check if `credentials: 'include'` is set in fetch requests
- Verify cookies are being set by backend (check browser DevTools → Application → Cookies)
- Check CORS configuration allows credentials

### Issue: 401 Unauthorized even after login

**Solution:**
- Verify access token cookie is being sent
- Check if token expired (access tokens expire in 15 minutes)
- Try calling `/api/auth/refresh` to get new token

### Issue: Refresh token not working

**Solution:**
- Verify refresh token cookie exists
- Check refresh token expiration (7 days)
- Verify Redis is running (refresh tokens stored in Redis)

---

## Example: Complete Auth Hook

```javascript
// hooks/useAuth.js

import { useState, useEffect, createContext, useContext } from 'react';
import { useRouter } from 'next/router';
import { initializeAuth } from '../utils/auth';

const AuthContext = createContext();

export function AuthProvider({ children }) {
  const [user, setUser] = useState(null);
  const [loading, setLoading] = useState(true);
  const router = useRouter();

  useEffect(() => {
    async function loadUser() {
      const authenticatedUser = await initializeAuth();
      setUser(authenticatedUser);
      setLoading(false);
    }

    loadUser();
  }, []);

  const logout = async () => {
    await fetch('http://localhost:8080/api/auth/logout', {
      method: 'POST',
      credentials: 'include',
    });
    setUser(null);
    router.push('/login');
  };

  return (
    <AuthContext.Provider value={{ user, loading, logout }}>
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth() {
  const context = useContext(AuthContext);
  if (!context) {
    throw new Error('useAuth must be used within AuthProvider');
  }
  return context;
}
```

**Usage:**
```javascript
// Wrap your app
<AuthProvider>
  <App />
</AuthProvider>

// In components
const { user, loading, logout } = useAuth();
```

---

## Summary

1. **On page load/refresh:** Call `GET /api/auth/me`
2. **If 401:** Call `POST /api/auth/refresh`, then retry `/api/auth/me`
3. **If refresh fails:** Redirect to login
4. **Always use:** `credentials: 'include'` in fetch requests
5. **Validate user type:** After getting user, verify they can access the dashboard

This ensures users remain logged in when refreshing the page!

