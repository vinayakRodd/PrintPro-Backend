# Frontend API Code for Google Sign-In

## API Endpoint
```
POST http://localhost:8080/api/auth/google/signin
```

## Request Body
```json
{
  "token": "google-id-token-here"
}
```

## Response (Success)
```json
{
  "success": true,
  "message": "User authenticated successfully",
  "user": {
    "id": "google-user-id",
    "email": "user@example.com",
    "name": "John Doe",
    "picture": "https://...",
    "provider": "google",
    "created_at": "2025-12-26T10:00:00Z",
    "updated_at": "2025-12-26T10:00:00Z"
  },
  "token": "session-token-here"
}
```

## Response (Error)
```json
{
  "success": false,
  "message": "Error message",
  "error": "Detailed error description"
}
```

---

## JavaScript/TypeScript Code

### Option 1: Using Fetch API

```javascript
// googleSignInService.js
const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080';

export const signInWithGoogle = async (idToken) => {
  try {
    const response = await fetch(`${API_BASE_URL}/api/auth/google/signin`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({
        token: idToken,
      }),
    });

    const data = await response.json();

    if (!response.ok || !data.success) {
      throw new Error(data.message || 'Authentication failed');
    }

    return {
      success: true,
      user: data.user,
      token: data.token,
    };
  } catch (error) {
    console.error('Google Sign-In Error:', error);
    return {
      success: false,
      error: error.message || 'Failed to authenticate with Google',
    };
  }
};
```

### Option 2: Using Axios

```javascript
// googleSignInService.js
import axios from 'axios';

const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080';

export const signInWithGoogle = async (idToken) => {
  try {
    const response = await axios.post(
      `${API_BASE_URL}/api/auth/google/signin`,
      { token: idToken },
      {
        headers: {
          'Content-Type': 'application/json',
        },
      }
    );

    return {
      success: true,
      user: response.data.user,
      token: response.data.token,
    };
  } catch (error) {
    console.error('Google Sign-In Error:', error);
    return {
      success: false,
      error: error.response?.data?.message || error.message || 'Failed to authenticate',
    };
  }
};
```

---

## React/Next.js Component Example

```jsx
// components/GoogleSignInButton.jsx
'use client';

import { useGoogleLogin } from '@react-oauth/google';
import { signInWithGoogle } from '../services/googleSignInService';
import { useState } from 'react';

export default function GoogleSignInButton() {
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState(null);
  const [user, setUser] = useState(null);

  const handleGoogleLogin = useGoogleLogin({
    onSuccess: async (tokenResponse) => {
      setLoading(true);
      setError(null);
      
      try {
        // Get the ID token from Google
        const idToken = tokenResponse.access_token;
        
        // Call your backend API
        const result = await signInWithGoogle(idToken);
        
        if (result.success) {
          setUser(result.user);
          // Store session token
          if (result.token) {
            localStorage.setItem('sessionToken', result.token);
          }
          console.log('User authenticated:', result.user);
        } else {
          setError(result.error);
        }
      } catch (err) {
        setError(err.message);
      } finally {
        setLoading(false);
      }
    },
    onError: (error) => {
      console.error('Google Login Error:', error);
      setError('Google login failed');
    },
  });

  return (
    <div>
      <button
        onClick={() => handleGoogleLogin()}
        disabled={loading}
        className="px-4 py-2 bg-blue-500 text-white rounded"
      >
        {loading ? 'Signing in...' : 'Sign in with Google'}
      </button>
      
      {error && <p className="text-red-500 mt-2">{error}</p>}
      
      {user && (
        <div className="mt-4">
          <p>Welcome, {user.name}!</p>
          <p>Email: {user.email}</p>
        </div>
      )}
    </div>
  );
}
```

---

## Alternative: Using Google Identity Services (gsi)

```javascript
// If using Google Identity Services directly
function handleCredentialResponse(response) {
  const idToken = response.credential;
  
  // Call your backend API
  fetch('http://localhost:8080/api/auth/google/signin', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({ token: idToken }),
  })
    .then(res => res.json())
    .then(data => {
      if (data.success) {
        console.log('User:', data.user);
        localStorage.setItem('sessionToken', data.token);
      } else {
        console.error('Error:', data.message);
      }
    })
    .catch(error => {
      console.error('API Error:', error);
    });
}
```

---

## Quick Test with cURL

```bash
curl -X POST http://localhost:8080/api/auth/google/signin \
  -H "Content-Type: application/json" \
  -d '{"token": "your-google-id-token-here"}'
```

---

## Environment Variables

Create `.env.local` in your frontend project:

```
NEXT_PUBLIC_API_URL=http://localhost:8080
```

