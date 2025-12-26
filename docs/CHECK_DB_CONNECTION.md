# How to Check Database Connection

## Method 1: Health Check Endpoint (Recommended)

The `/health` and `/ping` endpoints now include database connectivity checks.

### Check via Browser

Open your browser and navigate to:
```
http://localhost:8080/health
```

or

```
http://localhost:8080/ping
```

### Check via Command Line (PowerShell)

```powershell
# Using Invoke-WebRequest
Invoke-WebRequest -Uri http://localhost:8080/health | Select-Object -ExpandProperty Content

# Using curl (if available)
curl http://localhost:8080/health
```

### Expected Response (All Connected)

```json
{
  "status": "ok",
  "message": "Server is responding",
  "timestamp": "2024-01-15T10:30:00Z",
  "redis": {
    "status": "connected"
  },
  "postgres": {
    "status": "connected"
  }
}
```

### Expected Response (Database Disconnected)

```json
{
  "status": "degraded",
  "message": "Server is responding",
  "timestamp": "2024-01-15T10:30:00Z",
  "redis": {
    "status": "disconnected"
  },
  "postgres": {
    "status": "disconnected"
  }
}
```

**HTTP Status Codes:**
- `200 OK` - All services connected
- `503 Service Unavailable` - One or more services disconnected

---

## Method 2: Check Server Logs

When the server starts, you'll see connection messages:

### Successful Connection
```
Connected to Redis at localhost:6379
Connected to PostgreSQL database: printer_db
```

### Failed Connection
```
Failed to connect to PostgreSQL: connection refused
```
or
```
Failed to connect to PostgreSQL: password authentication failed
```

---

## Method 3: Test Connection Directly

### Test PostgreSQL Connection

```powershell
# Using psql (if installed)
psql -U postgres -d printer_db -h localhost -p 5432

# Or test connection string
psql "postgres://postgres:your_password@localhost:5432/printer_db?sslmode=disable"
```

### Test Redis Connection

```powershell
# Using redis-cli (if installed)
redis-cli -h localhost -p 6379 ping
# Should return: PONG
```

---

## Method 4: Programmatic Check

You can also check programmatically in your code:

```go
// Check PostgreSQL
ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
defer cancel()

if err := postgresClient.Ping(ctx); err != nil {
    log.Printf("PostgreSQL disconnected: %v", err)
} else {
    log.Printf("PostgreSQL connected")
}

// Check Redis
if err := redisClient.GetClient().Ping(ctx).Err(); err != nil {
    log.Printf("Redis disconnected: %v", err)
} else {
    log.Printf("Redis connected")
}
```

---

## Troubleshooting

### PostgreSQL Not Connecting

1. **Check if PostgreSQL is running:**
   ```powershell
   # Windows
   Get-Service postgresql*
   ```

2. **Verify environment variables in `.env`:**
   ```env
   POSTGRES_HOST=localhost
   POSTGRES_PORT=5432
   POSTGRES_USER=postgres
   POSTGRES_PASSWORD=your_password
   POSTGRES_DB=printer_db
   ```

3. **Check if database exists:**
   ```sql
   psql -U postgres -c "\l"  # List all databases
   ```

4. **Test connection manually:**
   ```powershell
   psql -U postgres -d printer_db
   ```

### Redis Not Connecting

1. **Check if Redis is running:**
   ```powershell
   # Windows (if installed as service)
   Get-Service redis*
   ```

2. **Verify environment variables:**
   ```env
   REDIS_ADDR=localhost:6379
   REDIS_PASSWORD=
   REDIS_DB=0
   ```

3. **Test connection manually:**
   ```powershell
   redis-cli ping
   ```

---

## Quick Test Script

Create a simple test script to check both databases:

```powershell
# test-connections.ps1
Write-Host "Testing Database Connections..." -ForegroundColor Cyan

# Test Health Endpoint
$response = Invoke-WebRequest -Uri http://localhost:8080/health -UseBasicParsing
$health = $response.Content | ConvertFrom-Json

Write-Host "`nHealth Status: $($health.status)" -ForegroundColor $(if ($health.status -eq "ok") { "Green" } else { "Red" })
Write-Host "Redis: $($health.redis.status)" -ForegroundColor $(if ($health.redis.status -eq "connected") { "Green" } else { "Red" })
Write-Host "PostgreSQL: $($health.postgres.status)" -ForegroundColor $(if ($health.postgres.status -eq "connected") { "Green" } else { "Red" })
```

Run it:
```powershell
.\test-connections.ps1
```

---

## Summary

| Method | When to Use |
|--------|-------------|
| **Health Endpoint** | Quick check, monitoring, automated tests |
| **Server Logs** | During startup, debugging connection issues |
| **Direct CLI** | Manual verification, troubleshooting |
| **Programmatic** | Custom checks, integration tests |

The health endpoint is the easiest way to check database connectivity in real-time!

