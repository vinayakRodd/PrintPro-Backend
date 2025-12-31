# Identity-Profile Pattern Implementation

## Overview

The authentication system has been refactored to use the **Identity-Profile Pattern**, which separates authentication (identity) from user-specific data (profiles). This provides better separation of concerns and scalability.

## Database Schema

### Accounts Table (Identity)
Central table for all authentication:
- `id` (SERIAL PRIMARY KEY)
- `email` (VARCHAR(255) UNIQUE NOT NULL)
- `password_hash` (TEXT NOT NULL)
- `user_type` (VARCHAR(20) NOT NULL) - 'partner' or 'customer'
- `created_at` (TIMESTAMP)

### Partner Profiles Table
Stores partner-specific information:
- `id` (SERIAL PRIMARY KEY)
- `account_id` (INT NOT NULL, FK to accounts.id)
- `shop_name` (VARCHAR(100) NOT NULL)
- `printer_id` (VARCHAR(50)) - Unique ID for Python Agent
- Foreign Key with `ON DELETE CASCADE`

### Customer Profiles Table
Stores customer-specific information:
- `id` (SERIAL PRIMARY KEY)
- `account_id` (INT NOT NULL, FK to accounts.id)
- `phone_number` (VARCHAR(20))
- `wallet_balance` (DECIMAL(10,2) DEFAULT 0.00)
- Foreign Key with `ON DELETE CASCADE`

## Registration Endpoints

### Partner Registration
**Endpoint:** `POST /api/auth/register/partner`

**Request:**
```json
{
  "email": "shop@example.com",
  "password": "password123",
  "shop_name": "Print Pro Shop",
  "printer_id": "SHOP_001"
}
```

**Flow:**
1. Start database transaction
2. Create account in `accounts` table with `user_type = 'partner'`
3. Create profile in `partner_profiles` table using generated `account_id`
4. Commit transaction (or rollback on error)

### Customer Registration
**Endpoint:** `POST /api/auth/register/customer`

**Request:**
```json
{
  "email": "customer@example.com",
  "password": "password123",
  "phone_number": "+1234567890"
}
```

**Flow:**
1. Start database transaction
2. Create account in `accounts` table with `user_type = 'customer'`
3. Create profile in `customer_profiles` table using generated `account_id`
4. Commit transaction (or rollback on error)

## Login Flow

**Endpoint:** `POST /api/auth/login`

**Request:**
```json
{
  "email": "user@example.com",
  "password": "password123"
}
```

**Flow:**
1. Query `accounts` table by email
2. Verify password using bcrypt
3. Extract `user_type` from account record
4. Generate JWT tokens with `user_type` in claims
5. Return access token (refresh token in HTTP-only cookie)

**Note:** The frontend does NOT need to send `user_type` - it's automatically detected from the database.

## JWT Token Claims

JWT tokens now include:
- `user_id`: Account ID
- `email`: Account email
- `user_type`: "partner" or "customer"
- `token_type`: "access" or "refresh"

## Role-Based Authorization

### RequireRole Middleware

**Location:** `internal/middleware/role/role.go`

**Usage:**
```go
role.RequireRole("partner")(handler)
```

**How it works:**
1. Extracts `user_type` from request context (set by auth middleware)
2. Compares with required role
3. Returns 403 Forbidden if roles don't match

### Protected Routes

**Printer Update Route:**
```go
http.HandleFunc("/api/update-printers", 
    corsHandler(
        rateLimiter.LimitMiddleware(
            authMiddlewareFunc(
                role.RequireRole("partner")(
                    printer_handler.UpdatePrintersHandler
                )
            )
        )
    )
)
```

This route requires:
1. Valid JWT access token (auth middleware)
2. User type must be "partner" (role middleware)

## Migration

Run the migration to create the new tables:
```sql
-- See migrations/002_create_identity_profile_tables.sql
```

**Important:** This migration creates new tables. Existing `users` and `partners` tables remain for backward compatibility but new registrations use the Identity-Profile pattern.

## Code Structure

### Models
- `internal/models/account/account.go` - Account model
- `internal/models/partner_profile/partner_profile.go` - Partner profile model
- `internal/models/customer_profile/customer_profile.go` - Customer profile model

### Repositories
- `internal/repositories/account_repository.go` - Account CRUD operations
- `internal/repositories/partner_profile_repository.go` - Partner profile operations
- `internal/repositories/customer_profile_repository.go` - Customer profile operations

### Handlers
- `internal/api/handlers/auth_handler/register_handler.go` - Partner and Customer registration
- `internal/api/handlers/auth_handler/login_handler.go` - Updated to use accounts table

### Middleware
- `internal/middleware/auth_middleware/auth.go` - JWT validation (stores user_type in context)
- `internal/middleware/role/role.go` - Role-based authorization

## Security Features

1. **Transaction Safety:** Registration uses database transactions to ensure atomicity
2. **Password Hashing:** bcrypt with default cost
3. **JWT Security:** Access tokens (15 min) and refresh tokens (7 days)
4. **Role-Based Access:** Middleware enforces role requirements
5. **SQL Injection Prevention:** Prepared statements via pgxpool

## Backward Compatibility

- Old `/api/auth/register` endpoint still works (creates customer accounts)
- Existing `users` and `partners` tables remain
- Google Sign-In still uses old user table (TODO: migrate to accounts)

## Next Steps

1. Migrate Google Sign-In to use accounts table
2. Update password reset flow to use accounts table
3. Add migration script to move existing users/partners to accounts + profiles
4. Add profile update endpoints
5. Add partner profile management endpoints

