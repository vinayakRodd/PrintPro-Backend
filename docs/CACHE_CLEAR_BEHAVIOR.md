# What Happens When You Clear Cache

## Scenario 1: Clear Browser Cache (Cookies)

### What Gets Cleared:
- Browser cookies (including `session_token` cookie)
- Local storage
- Session storage

### What Happens:

1. **Cookie is Deleted:**
   - The `session_token` cookie is removed from browser
   - Browser no longer sends cookie with requests

2. **When User Calls `/api/auth/me`:**
   ```
   Request → No cookie found → 401 Unauthorized
   ```
   - Middleware checks for cookie: ❌ Not found
   - Returns: `{"success": false, "message": "Unauthorized", "error": "No session token found"}`
   - **No Redis lookup happens** (because no cookie to check)

3. **Session Still Exists in Redis:**
   - ✅ Session data is still in Redis
   - ✅ Session is still valid (7 days TTL)
   - ❌ But user can't access it (no cookie to authenticate)

4. **User Must Sign In Again:**
   - User needs to sign in via Google again
   - New session token generated
   - New cookie set in browser
   - New session created in Redis (old one remains until expiry)

### Result:
- **User is logged out** (can't authenticate)
- **Old session still in Redis** (wasted space until expiry)
- **Must sign in again** to get new session

---

## Scenario 2: Clear Redis Cache

### What Gets Cleared:
- All Redis data (sessions, rate limits, etc.)
- All `session:*` keys deleted

### What Happens:

1. **All Sessions Deleted:**
   - All `session:{token}` keys removed from Redis
   - All user sessions invalidated

2. **When User Calls `/api/auth/me`:**
   ```
   Request → Cookie found → Redis lookup → ❌ Not found → 401 Unauthorized
   ```
   - Middleware finds cookie ✅
   - Looks up session in Redis: ❌ Not found
   - Returns: `{"success": false, "message": "Unauthorized", "error": "Invalid or expired session"}`
   - Log: No message (session not found)

3. **All Users Logged Out:**
   - Every user's session is invalid
   - All users must sign in again

4. **Rate Limits Reset:**
   - All rate limit counters reset
   - Users can make fresh requests

### Result:
- **All users logged out** immediately
- **All sessions invalidated**
- **Everyone must sign in again**

---

## Scenario 3: Clear Both (Browser + Redis)

### What Happens:
1. Cookie deleted from browser
2. Sessions deleted from Redis
3. Complete reset - no authentication possible
4. All users must sign in again

---

## How to Clear Redis Cache

### Clear All Data:
```bash
redis-cli FLUSHALL
```

### Clear Only Sessions:
```bash
redis-cli
KEYS session:*
# Then delete each one, or:
redis-cli --scan --pattern "session:*" | xargs redis-cli DEL
```

### Clear Rate Limits:
```bash
redis-cli
KEYS ratelimit:*
# Delete as needed
```

---

## Best Practices

1. **Don't Clear Redis in Production:**
   - Will log out all users
   - Causes poor user experience

2. **Session Expiration:**
   - Sessions auto-expire after 7 days
   - No manual cleanup needed

3. **If User Clears Browser Cache:**
   - They're logged out (expected behavior)
   - They can sign in again
   - Old session in Redis expires naturally

4. **Monitoring:**
   - Check Redis memory usage
   - Monitor session count: `KEYS session:* | wc -l`
   - Sessions auto-cleanup after TTL expires

---

## Summary

| Action | Cookie Status | Redis Status | User Status |
|--------|--------------|--------------|-------------|
| Clear Browser Cache | ❌ Deleted | ✅ Still exists | ❌ Logged out |
| Clear Redis Cache | ✅ Still exists | ❌ Deleted | ❌ Logged out |
| Clear Both | ❌ Deleted | ❌ Deleted | ❌ Logged out |
| Normal Operation | ✅ Exists | ✅ Exists | ✅ Logged in |

**Key Point:** Both cookie AND Redis session must exist for authentication to work!

