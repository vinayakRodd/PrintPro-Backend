# Password Reset API Documentation

## Overview

The password reset flow uses OTP (One-Time Password) sent via email. The OTP expires after 5 minutes and can only be used once.

## Flow Diagram

```
User Requests Password Reset
    ↓
POST /api/auth/forgot-password
    ↓
Check if user exists
    ↓
Generate 6-digit OTP
    ↓
Store OTP in Redis (5 min TTL)
    ↓
Send OTP via Gmail SMTP
    ↓
User Receives Email with OTP
    ↓
User Submits OTP + New Password
    ↓
POST /api/auth/reset-password
    ↓
Verify OTP (check Redis)
    ↓
Hash New Password (bcrypt)
    ↓
Update Password in Database
    ↓
Delete OTP from Redis
    ↓
Password Reset Complete
```

## API Endpoints

### 1. Request Password Reset (Send OTP)

**Endpoint:** `POST /api/auth/forgot-password`

**Request Body:**
```json
{
  "email": "user@example.com"
}
```

**Success Response (200 OK):**
```json
{
  "success": true,
  "message": "OTP has been sent to your email. It will expire in 5 minutes."
}
```

**Error Responses:**
- `400 Bad Request` - Invalid email format
- `500 Internal Server Error` - Failed to send email

**Security Note:** The API always returns success even if the email doesn't exist to prevent email enumeration attacks.

---

### 2. Reset Password (Verify OTP)

**Endpoint:** `POST /api/auth/reset-password`

**Request Body:**
```json
{
  "email": "user@example.com",
  "otp": "123456",
  "password": "newSecurePassword123"
}
```

**Success Response (200 OK):**
```json
{
  "success": true,
  "message": "Password has been reset successfully"
}
```

**Error Responses:**
- `400 Bad Request` - Missing fields or password too short (< 6 characters)
- `401 Unauthorized` - Invalid or expired OTP
- `404 Not Found` - User not found
- `400 Bad Request` - OAuth users cannot reset password
- `500 Internal Server Error` - Failed to update password

---

## Environment Variables

Add these to your `.env` file for Gmail SMTP:

```env
# Gmail SMTP Configuration
SMTP_HOST=smtp.gmail.com
SMTP_PORT=587
SMTP_USERNAME=your-email@gmail.com
SMTP_PASSWORD=your-app-password
FROM_EMAIL=your-email@gmail.com  # This is the "from" email address (required)
```

**Important Notes:**
- `SMTP_USERNAME`: Your Gmail address (used for authentication)
- `SMTP_PASSWORD`: Your Gmail App Password (16 characters)
- `FROM_EMAIL`: The email address that will appear as the sender
  - If not set, it will use `SMTP_USERNAME` as the sender
  - This is the email address users will see in their inbox
  - Should match your Gmail address or be an alias you control

### Getting Gmail App Password

1. Go to [Google Account Settings](https://myaccount.google.com/)
2. Enable 2-Step Verification
3. Go to "App passwords"
4. Generate a new app password for "Mail"
5. Use the 16-character password as `SMTP_PASSWORD`

**Important:** Don't use your regular Gmail password. Use an app-specific password.

---

## OTP Storage

OTPs are stored in Redis with the following structure:

```
Key: otp:{email}
Value: {6-digit-OTP}
TTL: 5 minutes
```

Example:
```
Key: otp:user@example.com
Value: 123456
TTL: 300 seconds (5 minutes)
```

**Security Features:**
- OTP expires after 5 minutes
- OTP is deleted after successful use (one-time use)
- OTP is case-sensitive

---

## Password Hashing

Passwords are hashed using **bcrypt** with default cost (10 rounds).

**Before Storage:**
```
Plain Password: "myPassword123"
    ↓
bcrypt.Hash()
    ↓
Hashed: "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"
```

**Security:**
- Passwords are never stored in plain text
- Each password gets a unique salt
- bcrypt is computationally expensive (prevents brute force)

---

## Example Usage

### Step 1: Request Password Reset

```bash
curl -X POST http://localhost:8080/api/auth/forgot-password \
  -H "Content-Type: application/json" \
  -d '{
    "email": "user@example.com"
  }'
```

**Response:**
```json
{
  "success": true,
  "message": "OTP has been sent to your email. It will expire in 5 minutes."
}
```

**Email Received:**
```
Subject: Password Reset OTP

Hello,

You have requested to reset your password. Please use the following OTP code:

123456

This OTP will expire in 5 minutes.

If you did not request this password reset, please ignore this email.

Best regards,
Print Pro Team
```

### Step 2: Reset Password with OTP

```bash
curl -X POST http://localhost:8080/api/auth/reset-password \
  -H "Content-Type: application/json" \
  -d '{
    "email": "user@example.com",
    "otp": "123456",
    "password": "newSecurePassword123"
  }'
```

**Response:**
```json
{
  "success": true,
  "message": "Password has been reset successfully"
}
```

---

## Error Scenarios

### Invalid OTP
```json
{
  "success": false,
  "message": "Invalid or expired OTP",
  "error": "The OTP is invalid or has expired. Please request a new one."
}
```

### Expired OTP
- OTP expires after 5 minutes
- User must request a new OTP

### OAuth User Attempt
```json
{
  "success": false,
  "message": "Cannot reset password",
  "error": "This account uses Google Sign-In. Please use Google to sign in."
}
```

### Password Too Short
```json
{
  "success": false,
  "message": "Password too short",
  "error": "Password must be at least 6 characters"
}
```

---

## Security Considerations

1. **Email Enumeration Prevention**: API always returns success even if email doesn't exist
2. **OTP Expiration**: OTPs expire after 5 minutes
3. **One-Time Use**: OTP is deleted after successful verification
4. **Password Hashing**: Passwords are hashed with bcrypt before storage
5. **OAuth Protection**: OAuth users cannot reset passwords (they use Google)
6. **Rate Limiting**: Endpoints are protected by rate limiting middleware

---

## Testing

### Test Password Reset Flow

1. **Request OTP:**
   ```bash
   POST /api/auth/forgot-password
   {"email": "test@example.com"}
   ```

2. **Check Email** - Should receive OTP

3. **Reset Password:**
   ```bash
   POST /api/auth/reset-password
   {
     "email": "test@example.com",
     "otp": "received-otp",
     "password": "newPassword123"
   }
   ```

4. **Verify in Database:**
   ```sql
   SELECT email, password_hash FROM users WHERE email = 'test@example.com';
   ```

### Test OTP Expiration

1. Request OTP
2. Wait 6 minutes
3. Try to use OTP → Should fail with "expired" error

### Test Invalid OTP

1. Request OTP
2. Use wrong OTP → Should fail with "invalid" error

---

## Troubleshooting

### Email Not Sending

1. **Check SMTP Configuration:**
   - Verify `SMTP_USERNAME` and `SMTP_PASSWORD` in `.env`
   - Ensure you're using Gmail App Password (not regular password)

2. **Check Gmail Settings:**
   - 2-Step Verification must be enabled
   - App Password must be generated

3. **Check Server Logs:**
   ```
   Failed to send OTP email: ...
   ```

### OTP Not Working

1. **Check Redis:**
   ```bash
   redis-cli
   KEYS otp:*
   GET otp:user@example.com
   TTL otp:user@example.com
   ```

2. **Verify OTP Format:**
   - Must be exactly 6 digits
   - Case-sensitive

### Password Update Failing

1. **Check Database Connection:**
   - Verify PostgreSQL is running
   - Check connection in health endpoint

2. **Check User Exists:**
   ```sql
   SELECT * FROM users WHERE email = 'user@example.com';
   ```

3. **Check if OAuth User:**
   - OAuth users have `password_hash = 'oauth_google'`
   - Cannot reset password for OAuth users

---

## Next Steps

- Add email templates (HTML emails)
- Add OTP resend functionality
- Add rate limiting for OTP requests
- Add password strength validation
- Add password history (prevent reusing old passwords)

