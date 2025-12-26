# PostgreSQL Setup Guide

## Overview

The application uses PostgreSQL for persistent data storage. The connection is managed through a connection pool using `pgxpool` for efficient database operations.

## Environment Variables

Add these variables to your `.env` file:

```env
# PostgreSQL Configuration
POSTGRES_HOST=localhost
POSTGRES_PORT=5432
POSTGRES_USER=postgres
POSTGRES_PASSWORD=your_password
POSTGRES_DB=printer_db
POSTGRES_SSLMODE=disable
```

### Environment Variable Details

| Variable | Default | Description |
|----------|---------|-------------|
| `POSTGRES_HOST` | `localhost` | PostgreSQL server hostname |
| `POSTGRES_PORT` | `5432` | PostgreSQL server port |
| `POSTGRES_USER` | `postgres` | PostgreSQL username |
| `POSTGRES_PASSWORD` | (empty) | PostgreSQL password (required) |
| `POSTGRES_DB` | `printer_db` | Database name |
| `POSTGRES_SSLMODE` | `disable` | SSL mode (use `require` in production) |

## Installation

### 1. Install PostgreSQL

**Windows:**
- Download from [PostgreSQL Downloads](https://www.postgresql.org/download/windows/)
- Or use Chocolatey: `choco install postgresql`

**macOS:**
```bash
brew install postgresql
brew services start postgresql
```

**Linux (Ubuntu/Debian):**
```bash
sudo apt-get update
sudo apt-get install postgresql postgresql-contrib
sudo systemctl start postgresql
```

### 2. Create Database

```bash
# Connect to PostgreSQL
psql -U postgres

# Create database
CREATE DATABASE printer_db;

# Exit
\q
```

### 3. Update .env File

Add your PostgreSQL password to `.env`:
```env
POSTGRES_PASSWORD=your_actual_password
```

## Connection String Format

The connection string is automatically built from environment variables:

```
postgres://{user}:{password}@{host}:{port}/{database}?sslmode={sslmode}
```

Example:
```
postgres://postgres:my_password@localhost:5432/printer_db?sslmode=disable
```

## Usage in Code

### Accessing the Database Pool

The PostgreSQL client is initialized in `main.go` and can be accessed through services:

```go
// In main.go
postgresClient, err := infrastructure.NewPostgresClient(postgresConnString)
if err != nil {
    log.Fatalf("Failed to connect to PostgreSQL: %v", err)
}
defer postgresClient.Close()

// Get the pool for database operations
pool := postgresClient.GetPool()

// Use the pool for queries
rows, err := pool.Query(ctx, "SELECT * FROM users WHERE id = $1", userID)
```

### Example: Using in a Service

```go
package services

import (
    "context"
    "print-pro-backend/internal/infrastructure"
    "github.com/jackc/pgx/v5/pgxpool"
)

type UserService struct {
    db *pgxpool.Pool
}

func NewUserService(postgresClient *infrastructure.PostgresClient) *UserService {
    return &UserService{
        db: postgresClient.GetPool(),
    }
}

func (s *UserService) GetUser(ctx context.Context, userID string) (*User, error) {
    var user User
    err := s.db.QueryRow(ctx, 
        "SELECT id, email, name FROM users WHERE id = $1", 
        userID,
    ).Scan(&user.ID, &user.Email, &user.Name)
    
    if err != nil {
        return nil, err
    }
    
    return &user, nil
}
```

## Connection Pool Features

- **Automatic Connection Management**: The pool manages connections efficiently
- **Connection Testing**: Automatically pings the database on initialization
- **Timeout Handling**: Connection attempts timeout after 5 seconds
- **Graceful Shutdown**: Connections are properly closed on application exit

## Troubleshooting

### Connection Failed

**Error:** `Failed to connect to PostgreSQL: connection refused`

**Solutions:**
1. Check if PostgreSQL is running:
   ```bash
   # Windows
   Get-Service postgresql*
   
   # macOS/Linux
   sudo systemctl status postgresql
   ```

2. Verify connection details in `.env` file
3. Check firewall settings
4. Ensure PostgreSQL is listening on the correct port

### Authentication Failed

**Error:** `password authentication failed`

**Solutions:**
1. Verify `POSTGRES_PASSWORD` in `.env` matches your PostgreSQL password
2. Check PostgreSQL user permissions:
   ```sql
   \du  -- List users
   ```

### Database Not Found

**Error:** `database "printer_db" does not exist`

**Solution:**
```sql
CREATE DATABASE printer_db;
```

## Production Considerations

1. **SSL Mode**: Change `POSTGRES_SSLMODE` to `require` or `verify-full`
2. **Connection Pooling**: Adjust pool size based on your application needs
3. **Password Security**: Use strong passwords and never commit `.env` files
4. **Backup Strategy**: Implement regular database backups
5. **Monitoring**: Monitor connection pool usage and database performance

## Next Steps

- Create database migrations/schema
- Implement user registration/login with database
- Add database models and repositories
- Set up database connection pooling configuration

