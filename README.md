# Print Pro Backend

A Go-based backend service for Print Pro application.

## Getting Started

### Prerequisites

- Go 1.21 or higher

### Installation

1. Clone the repository
2. Install dependencies:
   ```bash
   go mod download
   ```

### Running the Application

**Standard run (manual restart required):**
```bash
go run cmd/server/main.go
```

**Development with hot reload (auto-restart on file changes):**
```bash
air
```

The server will start on `http://localhost:8080`

> **Note:** If `air` command is not found, make sure your `$GOPATH/bin` or `$HOME/go/bin` is in your PATH, or use `go run $(go env GOPATH)/bin/air` instead.

### Building

```bash
go build -o print-pro-backend ./cmd/server
```

## Project Structure

```
.
├── cmd/
│   └── server/
│       └── main.go          # Main application entry point
├── internal/
│   ├── api/                 # API handlers and routes
│   ├── config/              # Configuration management
│   ├── constants/           # Application constants
│   ├── helpers/             # Helper functions
│   ├── infrastructure/      # Infrastructure setup (DB, cache, etc.)
│   ├── middleware/          # HTTP middleware
│   ├── models/              # Data models
│   ├── queries/             # Database queries
│   ├── repo/                # Repository layer
│   ├── server/              # Server setup and configuration
│   ├── services/            # Business logic services
│   ├── singleton/           # Singleton instances
│   ├── tools/               # Utility tools
│   └── utils/               # Utility functions
├── docs/                    # Documentation
├── .github/
│   └── workflows/           # GitHub Actions workflows
├── go.mod                   # Go module dependencies
└── README.md                # Project documentation
```

## Development

### Hot Reload Setup

This project uses [Air](https://github.com/air-verse/air) for automatic server restart on file changes. 

**Install Air (if not already installed):**
```bash
go install github.com/air-verse/air@latest
```

**Run with hot reload:**
```bash
air
```

Now any changes to `.go` files will automatically rebuild and restart the server.

This is the initial setup. More structure will be added as the project grows.

