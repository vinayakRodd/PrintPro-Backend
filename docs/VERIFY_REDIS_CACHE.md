# How to Verify Redis Cache is Working

## Method 1: Check Server Logs

When you call `/api/auth/me`, you'll see logs like:

```
Auth check: Looking up session in Redis for token: abc123...
Redis: Fetching session from cache - Key: session:abc123...
Redis: Session found in cache - Key: session:abc123...
Redis: Session retrieved successfully for user: user@example.com
Auth check: Session found in Redis for user: user@example.com
```

## Method 2: Use Redis CLI

### Connect to Redis
```bash
redis-cli
```

### Check if session exists
```bash
# List all session keys
KEYS session:*

# Get a specific session (replace with your actual token)
GET session:your-session-token-here

# Check TTL (time to live)
TTL session:your-session-token-here
```

### Monitor Redis commands in real-time
```bash
# In another terminal, run:
redis-cli MONITOR

# Then call /api/auth/me from your frontend
# You'll see all Redis commands being executed
```

## Method 3: Test with cURL

```bash
# 1. First, sign in to get a session token
curl -X POST http://localhost:8080/api/auth/google/signin \
  -H "Content-Type: application/json" \
  -d '{"token": "your-google-id-token"}' \
  -c cookies.txt

# 2. Check /api/auth/me (will use cookie from step 1)
curl -X GET http://localhost:8080/api/auth/me \
  -b cookies.txt \
  -v

# Watch server logs to see Redis operations
```

## Method 4: Check Redis Stats

```bash
redis-cli INFO stats
```

Look for:
- `keyspace_hits` - Number of successful key lookups
- `keyspace_misses` - Number of failed key lookups

## Expected Behavior

### When Session Exists:
- ✅ Server logs: "Redis: Session found in cache"
- ✅ Returns 200 with user data
- ✅ Redis `keyspace_hits` increases

### When Session Doesn't Exist:
- ❌ Server logs: "Redis: Session not found in cache"
- ❌ Returns 401 Unauthorized
- ✅ Redis `keyspace_misses` increases

## Debugging Tips

1. **Check Redis Connection:**
   ```bash
   redis-cli PING
   # Should return: PONG
   ```

2. **Check if Redis is Running:**
   ```bash
   # Windows
   Get-Service | Where-Object {$_.Name -like "*redis*"}
   
   # Linux/Mac
   redis-cli ping
   ```

3. **View All Sessions:**
   ```bash
   redis-cli
   KEYS session:*
   ```

4. **Check Session TTL:**
   ```bash
   redis-cli
   TTL session:your-token
   # Returns: -2 (doesn't exist), -1 (no expiry), or seconds remaining
   ```

## What to Look For

When `/api/auth/me` is called, you should see in server logs:

1. **Cookie extraction:**
   ```
   Auth check: Looking up session in Redis for token: ...
   ```

2. **Redis lookup:**
   ```
   Redis: Fetching session from cache - Key: session:...
   ```

3. **Result:**
   - If found: `Redis: Session found in cache`
   - If not found: `Redis: Session not found in cache`

This confirms Redis is being used for session verification!

