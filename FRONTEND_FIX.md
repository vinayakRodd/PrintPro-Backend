# Fix for Google Sign-In Token Issue

## Problem
The backend is receiving a 401 error because the frontend is sending an **access token** instead of an **ID token**.

## Solution

### Option 1: Use Google Identity Services (Recommended - Gets ID Token Directly)

Instead of `useGoogleLogin`, use Google Identity Services which provides the ID token directly:

```jsx
import { GoogleLogin } from '@react-oauth/google';

function GoogleSignInButton() {
  const handleSuccess = async (credentialResponse) => {
    // credentialResponse.credential is the ID token we need!
    const idToken = credentialResponse.credential;
    
    // Send to your backend
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
    }
  };

  return (
    <GoogleLogin
      onSuccess={handleSuccess}
      onError={() => {
        console.log('Login Failed');
      }}
    />
  );
}
```

### Option 2: Exchange Access Token for User Info (Current Approach)

If you're using `useGoogleLogin`, you get an access token. You need to:

1. Use the access token to get user info from Google
2. Send that info to your backend (backend needs to be updated)

**Frontend code:**
```javascript
import { useGoogleLogin } from '@react-oauth/google';

const handleGoogleLogin = useGoogleLogin({
  onSuccess: async (tokenResponse) => {
    // tokenResponse.access_token is an ACCESS TOKEN, not ID token
    
    // Option A: Get user info from Google and send to backend
    const userInfoResponse = await fetch(
      `https://www.googleapis.com/oauth2/v2/userinfo?access_token=${tokenResponse.access_token}`
    );
    const userInfo = await userInfoResponse.json();
    
    // Send user info to your backend
    // (Backend needs to be updated to accept this format)
    
    // Option B: Exchange for ID token (complex, not recommended)
  },
});
```

### Option 3: Update Backend to Accept Access Tokens

Update the backend to fetch user info using the access token instead of verifying an ID token.

## Recommended: Use Option 1 (Google Identity Services)

The `GoogleLogin` component from `@react-oauth/google` provides the ID token directly, which is what your backend expects.

## Quick Fix for Your Current Code

If you're using `useGoogleLogin`, change to `GoogleLogin` component:

```jsx
// Before (wrong - gives access token)
const handleGoogleLogin = useGoogleLogin({
  onSuccess: async (tokenResponse) => {
    const idToken = tokenResponse.access_token; // ❌ This is wrong!
    await signInWithGoogle(idToken);
  },
});

// After (correct - gives ID token)
<GoogleLogin
  onSuccess={(credentialResponse) => {
    const idToken = credentialResponse.credential; // ✅ This is correct!
    signInWithGoogle(idToken);
  }}
/>
```

