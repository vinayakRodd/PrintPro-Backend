# Google Sign-In API Setup

## Overview

This API endpoint allows users to sign in using their Google account. The endpoint verifies the Google ID token and registers/logs in the user.

## Configuration

### 1. Get Google OAuth Client ID

1. Go to [Google Cloud Console](https://console.cloud.google.com/)
2. Create a new project or select an existing one
3. Enable the Google+ API
4. Go to "Credentials" → "Create Credentials" → "OAuth client ID"
5. Configure the OAuth consent screen if prompted
6. Create an OAuth 2.0 Client ID
7. Copy the Client ID

### 2. Set Environment Variable

Set the `GOOGLE_CLIENT_ID` environment variable:

**Windows PowerShell:**
```powershell
$env:GOOGLE_CLIENT_ID="your-google-client-id-here"
```

**Windows CMD:**
```cmd
set GOOGLE_CLIENT_ID=your-google-client-id-here
```

**Linux/Mac:**
```bash
export GOOGLE_CLIENT_ID="your-google-client-id-here"
```

Or create a `.env` file in the root directory:
```
GOOGLE_CLIENT_ID=your-google-client-id-here
PORT=8080
```

## API Endpoint

### POST `/api/auth/google/signin`

Authenticates a user using Google Sign-In.

#### Request Body

```json
{
  "token": "google-id-token-here"
}
```

#### Success Response (200 OK)

```json
{
  "success": true,
  "message": "User authenticated successfully",
  "user": {
    "id": "google-user-id",
    "email": "user@example.com",
    "name": "John Doe",
    "picture": "https://lh3.googleusercontent.com/...",
    "provider": "google",
    "created_at": "2025-12-26T10:00:00Z",
    "updated_at": "2025-12-26T10:00:00Z"
  },
  "token": "session-token-here"
}
```

#### Error Response (400/401/500)

```json
{
  "success": false,
  "message": "Error message",
  "error": "Detailed error description"
}
```

## Testing with cURL

```bash
curl -X POST http://localhost:8080/api/auth/google/signin \
  -H "Content-Type: application/json" \
  -d '{"token": "your-google-id-token"}'
```

## Testing with PowerShell

```powershell
$body = @{
    token = "your-google-id-token"
} | ConvertTo-Json

Invoke-RestMethod -Uri "http://localhost:8080/api/auth/google/signin" `
  -Method Post `
  -ContentType "application/json" `
  -Body $body
```

## Frontend Integration

### JavaScript Example

```javascript
async function signInWithGoogle() {
  // Get Google ID token from your frontend Google Sign-In implementation
  const idToken = await getGoogleIdToken(); // Your frontend implementation
  
  const response = await fetch('http://localhost:8080/api/auth/google/signin', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({ token: idToken }),
  });
  
  const data = await response.json();
  
  if (data.success) {
    console.log('User authenticated:', data.user);
    // Store session token if needed
    localStorage.setItem('sessionToken', data.token);
  } else {
    console.error('Authentication failed:', data.message);
  }
}
```

## Notes

- The API verifies the Google ID token with Google's tokeninfo endpoint
- The client ID is validated against the token's audience
- Email verification status is checked
- Currently, user registration is a placeholder - you'll need to implement database logic in `internal/services/google_auth.go` → `RegisterOrLoginUser` method

