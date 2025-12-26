# Database Models Documentation

## Overview

This document describes the database models used in the Print Pro Backend application. Each model is defined in its own package within `internal/models/`.

## Model Structure

```
internal/models/
├── user/
│   └── user.go          # User model and request/response types
├── partner/
│   └── partner.go       # Partner (shop owner) model
├── printer/
│   └── printer.go       # Printer model
└── printjob/
    └── printjob.go      # Print job model
```

---

## User Model

**Package:** `internal/models/user`

**Database Table:** `users`

### Fields

| Field | Type | Description |
|-------|------|-------------|
| `id` | `int` | Primary key (SERIAL) |
| `full_name` | `string` | User's full name |
| `email` | `string` | Unique email address |
| `password_hash` | `string` | Hashed password (never exposed in JSON) |
| `created_at` | `time.Time` | Account creation timestamp |

### Usage

```go
import "print-pro-backend/internal/models/user"

// Create a new user
newUser := &user.User{
    FullName:     "John Doe",
    Email:        "john@example.com",
    PasswordHash: hashedPassword,
}
```

---

## Partner Model

**Package:** `internal/models/partner`

**Database Table:** `partners`

### Fields

| Field | Type | Description |
|-------|------|-------------|
| `id` | `int` | Primary key (SERIAL) |
| `owner_name` | `string` | Partner's (shop owner) name |
| `shop_name` | `string` | Name of the print shop |
| `email` | `string` | Unique email address |
| `password_hash` | `string` | Hashed password (never exposed in JSON) |
| `shop_address` | `string` | Physical address of the shop |
| `latitude` | `*float64` | GPS latitude (nullable) |
| `longitude` | `*float64` | GPS longitude (nullable) |
| `created_at` | `time.Time` | Account creation timestamp |

### Usage

```go
import "print-pro-backend/internal/models/partner"

// Create a new partner
newPartner := &partner.Partner{
    OwnerName:    "Jane Smith",
    ShopName:     "Quick Print Shop",
    Email:        "jane@shop.com",
    PasswordHash: hashedPassword,
    ShopAddress:  "123 Main St, City",
    Latitude:     &lat,
    Longitude:    &lng,
}
```

---

## Printer Model

**Package:** `internal/models/printer`

**Database Table:** `printers`

### Fields

| Field | Type | Description |
|-------|------|-------------|
| `id` | `int` | Primary key (SERIAL) |
| `partner_id` | `int` | Foreign key to `partners.id` |
| `model_name` | `string` | Printer model name |
| `is_color` | `bool` | Whether printer supports color printing |
| `status` | `string` | Printer status (see constants below) |
| `price_per_page` | `float64` | Cost per page in currency units |

### Status Constants

```go
printer.StatusAvailable   // "available"
printer.StatusBusy        // "busy"
printer.StatusMaintenance // "maintenance"
printer.StatusOffline     // "offline"
```

### Usage

```go
import "print-pro-backend/internal/models/printer"

// Create a new printer
newPrinter := &printer.Printer{
    PartnerID:    1,
    ModelName:    "HP LaserJet Pro",
    IsColor:      true,
    Status:       printer.StatusAvailable,
    PricePerPage: 0.10,
}
```

---

## PrintJob Model

**Package:** `internal/models/printjob`

**Database Table:** `print_jobs`

### Fields

| Field | Type | Description |
|-------|------|-------------|
| `id` | `int` | Primary key (SERIAL) |
| `user_id` | `*int` | Foreign key to `users.id` (nullable for guest printing) |
| `partner_id` | `int` | Foreign key to `partners.id` |
| `printer_id` | `*int` | Foreign key to `printers.id` (nullable, assigned later) |
| `filename` | `string` | Original filename |
| `file_url` | `string` | URL to the file to be printed |
| `status` | `string` | Job status (see constants below) |
| `total_cost` | `*float64` | Total cost (nullable, calculated later) |
| `created_at` | `time.Time` | Job creation timestamp |

### Status Constants

```go
printjob.StatusPending    // "pending"
printjob.StatusProcessing // "processing"
printjob.StatusCompleted  // "completed"
printjob.StatusFailed     // "failed"
printjob.StatusCancelled  // "cancelled"
```

### Usage

```go
import "print-pro-backend/internal/models/printjob"

// Create a new print job
userID := 1
newJob := &printjob.PrintJob{
    UserID:    &userID,
    PartnerID: 1,
    Filename:  "document.pdf",
    FileURL:   "https://storage.example.com/files/document.pdf",
    Status:    printjob.StatusPending,
}
```

---

## Request/Response Types

Each model package includes request and response types for API operations:

### User
- `CreateUserRequest` - For creating new users
- `UpdateUserRequest` - For updating user information

### Partner
- `CreatePartnerRequest` - For creating new partners
- `UpdatePartnerRequest` - For updating partner information

### Printer
- `CreatePrinterRequest` - For creating new printers
- `UpdatePrinterRequest` - For updating printer information

### PrintJob
- `CreatePrintJobRequest` - For creating new print jobs
- `UpdatePrintJobRequest` - For updating print job status

---

## Database Relationships

```
users (1) ──< (0..*) print_jobs
partners (1) ──< (0..*) printers
partners (1) ──< (0..*) print_jobs
printers (1) ──< (0..*) print_jobs
```

### Foreign Key Constraints

- `printers.partner_id` → `partners.id` (CASCADE DELETE)
- `print_jobs.user_id` → `users.id` (SET NULL on delete)
- `print_jobs.partner_id` → `partners.id` (CASCADE DELETE)
- `print_jobs.printer_id` → `printers.id` (SET NULL on delete)

---

## Database Setup

Run the migration file to create all tables:

```bash
psql -U postgres -d printer_db -f migrations/001_create_tables.sql
```

Or manually execute the SQL in `migrations/001_create_tables.sql`.

---

## Notes

1. **Password Hashes**: Never expose `password_hash` fields in JSON responses (marked with `json:"-"`)

2. **Nullable Fields**: Use pointers (`*int`, `*float64`) for nullable database fields

3. **Timestamps**: All `created_at` fields are automatically set by the database

4. **Indexes**: Indexes are created on frequently queried fields (email, foreign keys, status)

5. **Cascade Deletes**: 
   - Deleting a partner deletes all their printers and print jobs
   - Deleting a user sets `print_jobs.user_id` to NULL
   - Deleting a printer sets `print_jobs.printer_id` to NULL

