# PDF Preview Authentication Guide

## Overview

The PDF preview endpoint (`GET /api/test-print/preview`) requires authentication. This document explains what the backend expects and how to properly authenticate requests from the frontend.

## Endpoint

**URL:** `GET /api/test-print/preview?filename=<filename>`

**Example:**
```
GET /api/test-print/preview?filename=document.pdf
```

## Authentication Requirements

### 1. Authorization Header (REQUIRED)

The backend expects a **JWT Bearer token** in the `Authorization` header.

**Format:**
```
Authorization: Bearer <access_token>
```

**Example:**
```
Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VyX2lkIjoiMTIzIiw...
```

### 2. User Type Requirement

The user must be a **partner** (`user_type: "partner"`). Customers cannot view PDFs.

### 3. Access Token

- Must be a valid JWT access token
- Token must not be expired (15 minutes TTL)
- Token must contain valid user information

## Frontend Implementation

### Option 1: Using Fetch API

```javascript
// Get the access token from your auth store/context
const accessToken = getAccessToken(); // Your function to get token

// Make the request
const response = await fetch(
  `/api/test-print/preview?filename=${encodeURIComponent(filename)}`,
  {
    method: 'GET',
    headers: {
      'Authorization': `Bearer ${accessToken}`,
      'Content-Type': 'application/json',
    },
    credentials: 'include', // Include cookies if needed
  }
);

if (response.ok) {
  // PDF will be returned as binary data
  // Use in iframe or open in new tab
} else if (response.status === 401) {
  // Token expired or invalid - refresh token
  await refreshAccessToken();
  // Retry the request
} else {
  console.error('Failed to load PDF:', response.statusText);
}
```

### Option 2: Using Axios

```javascript
import axios from 'axios';

// Configure axios interceptor to add token
axios.interceptors.request.use((config) => {
  const token = getAccessToken(); // Your function to get token
  if (token) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
});

// Make the request
try {
  const response = await axios.get(
    `/api/test-print/preview?filename=${encodeURIComponent(filename)}`,
    {
      responseType: 'blob', // Important for binary data
      withCredentials: true,
    }
  );
  
  // Create blob URL for iframe
  const blob = new Blob([response.data], { type: 'application/pdf' });
  const url = window.URL.createObjectURL(blob);
  
  // Use in iframe
  // <iframe src={url} />
} catch (error) {
  if (error.response?.status === 401) {
    // Token expired - refresh and retry
    await refreshAccessToken();
  }
}
```

### Option 3: Direct iframe (Recommended)

**For iframe usage, you need to pass the token in the URL or use a different approach:**

```typescript
// Method 1: Use iframe with token in URL (if backend supports it)
// Note: This requires backend to accept token as query parameter
<iframe 
  src={`/api/test-print/preview?filename=${encodeURIComponent(filename)}&token=${accessToken}`}
  className="w-full h-full"
/>

// Method 2: Fetch PDF and create blob URL (More secure)
const loadPDF = async (filename: string) => {
  try {
    const response = await fetch(
      `/api/test-print/preview?filename=${encodeURIComponent(filename)}`,
      {
        headers: {
          'Authorization': `Bearer ${getAccessToken()}`,
        },
        credentials: 'include',
      }
    );
    
    if (!response.ok) {
      throw new Error('Failed to load PDF');
    }
    
    const blob = await response.blob();
    const url = window.URL.createObjectURL(blob);
    return url;
  } catch (error) {
    console.error('Error loading PDF:', error);
    throw error;
  }
};

// Usage
const pdfUrl = await loadPDF('document.pdf');
<iframe src={pdfUrl} />
```

## Error Responses

### 401 Unauthorized

**Possible reasons:**
1. Missing `Authorization` header
2. Invalid token format (not "Bearer <token>")
3. Expired access token
4. Invalid JWT token

**Response:**
```json
{
  "success": false,
  "message": "Unauthorized",
  "error": "Invalid or expired access token"
}
```

**Solution:**
- Refresh the access token using `/api/auth/refresh`
- Retry the request with new token

### 403 Forbidden

**Reason:** User is not a partner (user_type is not "partner")

**Response:**
```json
{
  "success": false,
  "message": "Forbidden",
  "error": "Only partners can view PDF files"
}
```

**Solution:**
- Ensure user is logged in as a partner
- Check user type before making request

### 404 Not Found

**Reason:** PDF file doesn't exist in ready or processing folders

**Response:**
```json
{
  "success": false,
  "message": "File not found",
  "error": "File 'document.pdf' not found in ready or processing folders"
}
```

### 400 Bad Request

**Possible reasons:**
1. Missing filename parameter
2. Invalid file type (not PDF)
3. Invalid filename (path traversal attempt)

**Response:**
```json
{
  "success": false,
  "message": "Invalid file type",
  "error": "Only PDF files can be viewed"
}
```

## Complete Frontend Example

```typescript
// PDF Viewer Component
import { useState, useEffect } from 'react';

interface PDFViewerProps {
  filename: string;
  accessToken: string;
}

const PDFViewer: React.FC<PDFViewerProps> = ({ filename, accessToken }) => {
  const [pdfUrl, setPdfUrl] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const loadPDF = async () => {
      try {
        setLoading(true);
        setError(null);

        const response = await fetch(
          `/api/test-print/preview?filename=${encodeURIComponent(filename)}`,
          {
            method: 'GET',
            headers: {
              'Authorization': `Bearer ${accessToken}`,
            },
            credentials: 'include',
          }
        );

        if (response.status === 401) {
          throw new Error('Unauthorized - Please refresh your token');
        }

        if (response.status === 403) {
          throw new Error('Forbidden - Partner access required');
        }

        if (response.status === 404) {
          throw new Error('PDF file not found');
        }

        if (!response.ok) {
          throw new Error(`Failed to load PDF: ${response.statusText}`);
        }

        // Create blob URL
        const blob = await response.blob();
        const url = window.URL.createObjectURL(blob);
        setPdfUrl(url);
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Failed to load PDF');
      } finally {
        setLoading(false);
      }
    };

    if (filename && accessToken) {
      loadPDF();
    }

    // Cleanup blob URL on unmount
    return () => {
      if (pdfUrl) {
        window.URL.revokeObjectURL(pdfUrl);
      }
    };
  }, [filename, accessToken]);

  if (loading) {
    return <div>Loading PDF...</div>;
  }

  if (error) {
    return <div className="error">Error: {error}</div>;
  }

  if (!pdfUrl) {
    return null;
  }

  return (
    <iframe
      src={pdfUrl}
      className="w-full h-full"
      title={`PDF Viewer: ${filename}`}
    />
  );
};

export default PDFViewer;
```

## Token Refresh Flow

If you get a 401 error, implement token refresh:

```typescript
async function refreshAccessToken(): Promise<string | null> {
  try {
    const response = await fetch('/api/auth/refresh', {
      method: 'POST',
      credentials: 'include', // Send refresh token cookie
    });

    if (response.ok) {
      const data = await response.json();
      // Store new access token
      setAccessToken(data.token);
      return data.token;
    }
  } catch (error) {
    console.error('Failed to refresh token:', error);
    // Redirect to login
    window.location.href = '/login';
  }
  return null;
}

// Usage in PDF loader
const loadPDFWithRetry = async (filename: string) => {
  let token = getAccessToken();
  
  let response = await fetch(
    `/api/test-print/preview?filename=${encodeURIComponent(filename)}`,
    {
      headers: { 'Authorization': `Bearer ${token}` },
      credentials: 'include',
    }
  );

  // If 401, refresh token and retry
  if (response.status === 401) {
    token = await refreshAccessToken();
    if (token) {
      response = await fetch(
        `/api/test-print/preview?filename=${encodeURIComponent(filename)}`,
        {
          headers: { 'Authorization': `Bearer ${token}` },
          credentials: 'include',
        }
      );
    }
  }

  return response;
};
```

## Important Notes

1. **Authorization Header is Required**: The backend will return 401 if the `Authorization` header is missing or invalid.

2. **Bearer Token Format**: The token must be prefixed with "Bearer " (with a space).

3. **Token Expiration**: Access tokens expire after 15 minutes. Implement token refresh logic.

4. **User Type**: Only users with `user_type: "partner"` can access this endpoint.

5. **CORS**: Ensure your frontend origin is allowed in CORS configuration.

6. **File Location**: PDFs must be in either:
   - `test-print/ready/` folder
   - `test-print/processing/` folder

7. **Security**: The backend automatically:
   - Sanitizes filenames (prevents path traversal)
   - Validates file type (PDF only)
   - Checks file exists before serving
   - Verifies user permissions

## Testing

### Test with curl:

```bash
# Replace <access_token> with your actual JWT token
curl -X GET \
  "http://localhost:8080/api/test-print/preview?filename=document.pdf" \
  -H "Authorization: Bearer <access_token>" \
  -H "Content-Type: application/json" \
  --output preview.pdf
```

### Test in Browser Console:

```javascript
fetch('/api/test-print/preview?filename=document.pdf', {
  headers: {
    'Authorization': 'Bearer YOUR_ACCESS_TOKEN_HERE',
  },
  credentials: 'include',
})
.then(response => {
  if (response.ok) {
    return response.blob();
  }
  throw new Error('Failed to load PDF');
})
.then(blob => {
  const url = window.URL.createObjectURL(blob);
  window.open(url, '_blank');
});
```

## Troubleshooting

### Issue: "Unauthorized" error

**Check:**
1. Is `Authorization` header present?
2. Is token format correct? (`Bearer <token>`)
3. Is token expired? (Check token expiration time)
4. Is token valid? (Verify JWT signature)

**Solution:**
- Refresh the access token
- Ensure token is sent in every request
- Check token expiration time

### Issue: "Forbidden" error

**Check:**
1. Is user logged in as partner?
2. Does JWT token contain `user_type: "partner"`?

**Solution:**
- Verify user type in token claims
- Ensure partner user is logged in

### Issue: PDF doesn't load in iframe

**Possible reasons:**
1. CORS blocking the request
2. Token not being sent with iframe request
3. Content-Type mismatch

**Solution:**
- Use blob URL approach (fetch PDF, create blob, use in iframe)
- Ensure CORS is configured correctly
- Check browser console for errors

## Summary

**Backend Expects:**
- ✅ `Authorization: Bearer <access_token>` header
- ✅ Valid, non-expired JWT access token
- ✅ User with `user_type: "partner"`
- ✅ Valid PDF filename in query parameter

**Frontend Must:**
- ✅ Send Authorization header with every request
- ✅ Handle 401 errors (token refresh)
- ✅ Handle 403 errors (user type check)
- ✅ Use blob URL for iframe (if token can't be sent via iframe src)

