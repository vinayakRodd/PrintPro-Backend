# User Registration and Login Flow

## Overview

When a user signs in with Google OAuth, the system automatically:
1. Verifies the Google ID token
2. Checks if the user exists in the database (by email)
3. Creates a new user if they don't exist (sign-up)
4. Returns the existing user if they do exist (login)
5. Creates a session and stores it in Redis

## Flow Diagram

```
User Signs In with Google
    ↓
Google ID Token Received
    ↓
Verify Token with Google API
    ↓
Extract User Info (email, name, etc.)
    ↓
Check Database: User exists by email?
    ├─ YES → Return existing user (LOGIN)
    └─ NO  → Create new user in database (SIGN-UP)
    ↓
Generate Session Token
    ↓
Store Session in Redis
    ↓
Set HTTP-only Cookie
    ↓
Return Success Response
```

## Database Storage

### User Table Structure

When a user signs in for the first time, a new record is created in the `users` table:

```sql
INSERT INTO users (full_name, email, password_hash, created_at)
VALUES ('John Doe', 'john@example.com', 'oauth_google', NOW())
```

**Important Notes:**
- `password_hash` is set to `"oauth_google"` as a marker (since OAuth users don't have passwords)
- The `password_hash` field is NOT NULL, so we use a placeholder
- In production, consider adding a `provider` field to track authentication method

### User Repository

The `UserRepository` (`internal/repositories/user_repository.go`) handles all database operations:

- **`GetByEmail(ctx, email)`** - Finds user by email
- **`Create(ctx, fullName, email, passwordHash)`** - Creates new user
- **`GetByID(ctx, id)`** - Retrieves user by database ID
- **`Update(ctx, id, fullName, email)`** - Updates user information

## Code Flow

### 1. Google Sign-In Request

**Endpoint:** `POST /api/auth/google/signin`

**Request:**
```json
{
  "token": "google_id_token_here"
}
```

### 2. Token Verification

The `GoogleAuthService.VerifyGoogleToken()` method:
- Validates the token with Google's API
- Verifies client ID, issuer, email verification
- Extracts user information (email, name, picture)

### 3. Register or Login

The `GoogleAuthService.RegisterOrLoginUser()` method:

```go
// Check if user exists
dbUser, err := userRepository.GetByEmail(ctx, email)
if err == nil {
    // User exists - return existing user (LOGIN)
    return existingUser, nil
}

// User doesn't exist - create new user (SIGN-UP)
newUser, err := userRepository.Create(ctx, name, email, "oauth_google")
return newUser, nil
```

### 4. Session Creation

After user is registered/logged in:
- Session token is generated
- User data is stored in Redis with 7-day TTL
- HTTP-only cookie is set in the browser

## Example: First Time User (Sign-Up)

1. User clicks "Sign in with Google"
2. Google authentication completes
3. Backend receives ID token
4. Token verified → User info extracted
5. Database check: User not found
6. **New user created in database:**
   ```sql
   id: 1
   full_name: "John Doe"
   email: "john@example.com"
   password_hash: "oauth_google"
   created_at: "2024-12-26 10:30:00"
   ```
7. Session created in Redis
8. Cookie set in browser
9. User logged in

## Example: Returning User (Login)

1. User clicks "Sign in with Google"
2. Google authentication completes
3. Backend receives ID token
4. Token verified → User info extracted
5. Database check: **User found** (email matches)
6. **Existing user returned** (no new record created)
7. Session created in Redis
8. Cookie set in browser
9. User logged in

## Session Storage

User sessions are stored in Redis with the following structure:

```
Key: session:{session_token}
Value: {
  "user_id": "1",
  "email": "john@example.com",
  "name": "John Doe",
  "created_at": 1703590200
}
TTL: 7 days
```

## User ID Mapping

- **Google ID** (from token): `"1234567890"` (string)
- **Database ID**: `1` (integer)
- **Session ID**: Database ID converted to string: `"1"`

The system uses the database ID as the primary identifier after registration.

## Error Handling

### User Already Exists
- If user exists, they are logged in (no error)
- No duplicate user creation

### Database Errors
- Connection failures return 500 error
- Unique constraint violations (email) are handled gracefully

### Token Verification Failures
- Invalid tokens return 401 Unauthorized
- Expired tokens are rejected
- Client ID mismatches are rejected

## Future Enhancements

1. **Provider Field**: Add `provider` column to track OAuth vs password auth
2. **Profile Updates**: Update user name if changed in Google profile
3. **Last Login**: Track last login timestamp
4. **Account Linking**: Link multiple OAuth providers to one account
5. **Email Verification**: Additional email verification for OAuth users

## Testing

### Test Sign-Up Flow
1. Use a new Google account
2. Sign in → Should create new user in database
3. Check database: `SELECT * FROM users WHERE email = 'test@example.com'`

### Test Login Flow
1. Use existing Google account
2. Sign in → Should return existing user
3. Check logs: Should see "User found" message

### Verify Database
```sql
-- Check all users
SELECT id, full_name, email, created_at FROM users;

-- Check specific user
SELECT * FROM users WHERE email = 'user@example.com';
```

## Security Notes

1. **Password Hash**: OAuth users have `"oauth_google"` as password_hash marker
2. **Session Tokens**: Cryptographically secure random tokens
3. **HTTP-only Cookies**: Prevents XSS attacks
4. **Token Verification**: All Google tokens are verified before use
5. **Email Uniqueness**: Database enforces unique email constraint

