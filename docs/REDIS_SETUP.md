# Redis Setup Guide

## Overview

Redis is integrated for:
1. **Session Management** - Store user sessions server-side
2. **Rate Limiting** - Limit API requests per client
3. **Caching** - Ready for future caching needs

## Installation

### Windows
Download Redis from: https://github.com/microsoftarchive/redis/releases
Or use WSL: `sudo apt-get install redis-server`

### Linux/Mac
```bash
# Ubuntu/Debian
sudo apt-get install redis-server

# Mac
brew install redis
```

### Docker (Recommended)
```bash
docker run -d -p 6379:6379 --name redis redis:latest
```

## Configuration

Add to your `.env` file:

```env
# Redis Configuration
REDIS_ADDR=localhost:6379
REDIS_PASSWORD=          # Leave empty if no password
REDIS_DB=0               # Database number (default 0)
```

## Features

### 1. Session Management
- Sessions stored in Redis with 7-day TTL
- Session key format: `session:{token}`
- Automatic expiration
- Session refresh capability

### 2. Rate Limiting
- **100 requests per minute** per client (configurable)
- Based on client IP address
- Returns `429 Too Many Requests` when exceeded
- Headers included:
  - `X-RateLimit-Limit`: Maximum requests
  - `X-RateLimit-Remaining`: Remaining requests

### 3. Session Operations

**Create Session:**
- Automatically created on successful Google Sign-In
- Stored in Redis with user data

**Get Session:**
```go
user, err := sessionService.GetSession(ctx, token)
```

**Delete Session:**
- Automatically deleted on logout
- Cookie also removed from browser

**Refresh Session:**
```go
err := sessionService.RefreshSession(ctx, token)
```

## Testing

1. Start Redis:
   ```bash
   redis-server
   # Or with Docker:
   docker start redis
   ```

2. Start your Go server:
   ```bash
   go run cmd/server/main.go
   ```

3. Check Redis connection:
   - Server will log: `Connected to Redis at localhost:6379`

4. Test rate limiting:
   - Make more than 100 requests in 1 minute
   - Should receive `429 Too Many Requests`

## Redis Commands (for debugging)

```bash
# Connect to Redis CLI
redis-cli

# View all session keys
KEYS session:*

# View rate limit keys
KEYS ratelimit:*

# Get session data
GET session:{your-token}

# Check TTL
TTL session:{your-token}

# Delete a key
DEL session:{your-token}
```

## Production Considerations

1. **Redis Password**: Set `REDIS_PASSWORD` in production
2. **Redis Persistence**: Configure Redis persistence (AOF/RDB)
3. **Redis Cluster**: Use Redis Cluster for high availability
4. **Connection Pooling**: Already handled by go-redis client
5. **Failover**: Rate limiter fails open (allows requests if Redis is down)

## Rate Limit Configuration

To change rate limits, modify in `cmd/server/main.go`:

```go
// Current: 100 requests per minute
rateLimiter := middleware.NewRateLimiter(redisClient, 100, time.Minute)

// Example: 50 requests per 30 seconds
rateLimiter := middleware.NewRateLimiter(redisClient, 50, 30*time.Second)
```

