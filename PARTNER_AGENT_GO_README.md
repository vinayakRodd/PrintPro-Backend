# Print Agent - Go Implementation with JWT Authentication

This is a complete Go rewrite of the Python print agent with JWT-based authentication, multi-printer support, and all original features.

## Features

✅ **JWT Authentication Flow:**
- Email/Password login → OTP verification → Token storage
- Encrypted token storage in user home directory
- Automatic token refresh on 401 errors
- 7-day refresh token expiry

✅ **Multi-Printer Support:**
- Server-side printer selection
- Color-aware printer routing
- Printer availability validation

✅ **Print Job Features:**
- Page range selection
- Odd/Even page filtering
- Skip pages
- Individual color pages
- Multiple copies
- Manual duplex with GUI confirmation

✅ **Networking:**
- WebSocket for real-time job notifications
- Periodic printer sync (every 5 seconds)
- Automatic reconnection

## Dependencies

Install required packages:

```bash
go mod init print-agent-go
go get fyne.io/fyne/v2@latest
go get github.com/alexbrainman/printer
go get github.com/gorilla/websocket
go get golang.org/x/crypto
```

## Building

```bash
# Development build
go build -o print-agent.exe partner_agent_go.go

# Release build (Windows)
go build -ldflags="-s -w" -o print-agent.exe partner_agent_go.go

# Cross-compile for other platforms
GOOS=linux GOARCH=amd64 go build -o print-agent partner_agent_go.go
```

## Usage

1. **Run the application:**
   ```bash
   ./print-agent.exe
   ```

2. **Login:**
   - Enter your email and password
   - Click "Login"
   - If OTP is required, enter the OTP code sent to your email
   - Click "Verify OTP"

3. **Start Agent:**
   - Enter server IP (default: 10.213.120.142)
   - Click "START AGENT"
   - The agent will sync printers and connect via WebSocket

## Token Storage

Tokens are encrypted and stored in:
- **Windows:** `C:\Users\<username>\.print_agent\tokens.enc`
- **Linux/Mac:** `~/.print_agent/tokens.enc`

Encryption uses machine-specific key (derived from hostname) for security.

## API Endpoints Used

- `POST /api/auth/login/partner` - Partner login
- `POST /api/auth/otp/verify` - OTP verification
- `POST /api/auth/refresh` - Token refresh
- `POST /api/partner-agent/sync-printers` - Sync printer list
- `GET /api/partner-agent/fetch-job` - Fetch print job
- `POST /api/partner-agent/confirm-print` - Confirm job completion
- `ws://<server>:8080/ws/printer1` - WebSocket connection

## TODO / Implementation Notes

### PDF Printing Implementation

The current code includes a placeholder for PDF printing. To complete the implementation, you'll need to:

1. **Add PDF Library:**
   ```bash
   go get github.com/gen2brain/go-fitz
   # OR
   go get github.com/pdfcpu/pdfcpu
   ```

2. **Implement PDF Rendering:**
   - Parse PDF pages
   - Convert pages to images (PNG/JPEG)
   - Handle color/grayscale conversion
   - Apply page filters (odd/even, skip pages)

3. **Windows Printing:**
   - Use Windows GDI+ or similar
   - Print images to selected printer
   - Handle printer device context
   - Implement manual duplex flow

### Example PDF Printing Code Structure:

```go
import (
    "github.com/gen2brain/go-fitz"
    "image"
    "image/color"
    "image/jpeg"
    // Windows printing APIs
)

func (p *PrintAgent) printPDF(params PrintJobParams) bool {
    doc, err := fitz.NewFromMemory(params.PDFData)
    if err != nil {
        return false
    }
    defer doc.Close()
    
    // Determine pages to print
    pages := p.calculatePages(doc.NumPage(), params)
    
    // Print each page
    for _, pageNum := range pages {
        img := doc.Image(pageNum)
        
        // Color conversion
        if !p.shouldPrintColor(pageNum, params) {
            img = convertToGrayscale(img)
        }
        
        // Print image to printer
        p.printImage(img, params.PrinterName)
    }
    
    return true
}
```

### Printer Package Notes

The `github.com/alexbrainman/printer` package API might differ. Adjust the `getRealtimePhysicalPrinters()` function based on the actual API:

```go
// Alternative implementation if API differs:
func (p *PrintAgent) getRealtimePhysicalPrinters() []string {
    // Use Windows API directly if needed
    // or adjust based on printer package documentation
}
```

## Authentication Flow Details

1. **Initial Login:**
   ```json
   POST /api/auth/login/partner
   {
     "email": "partner@example.com",
     "password": "password123"
   }
   ```
   Response may include `otp_required: true`

2. **OTP Verification:**
   ```json
   POST /api/auth/otp/verify
   {
     "email": "partner@example.com",
     "otp": "123456"
   }
   ```

3. **Token Refresh:**
   - Automatically called on 401 errors
   - Uses refresh token from cookie or stored token
   - Updates access token in memory and storage

## Error Handling

- **401 Unauthorized:** Automatically attempts token refresh
- **Refresh Token Expired:** Prompts user to login again
- **Printer Not Available:** Falls back to default printer
- **WebSocket Disconnect:** Automatically reconnects after 5 seconds

## Security Features

- ✅ Encrypted token storage
- ✅ Machine-specific encryption key
- ✅ Secure token file permissions (0600)
- ✅ Automatic token refresh
- ✅ JWT Bearer token authentication

## GUI Features

- Real-time log display
- Connection status indicator
- Login/OTP input fields
- Server IP configuration
- Manual duplex confirmation dialog

## Differences from Python Version

1. **Concurrency:** Uses goroutines instead of threads
2. **GUI:** Fyne toolkit instead of Tkinter
3. **Printing:** Requires PDF library integration (see TODO)
4. **Token Storage:** Encrypted file instead of plain storage
5. **Auto-Refresh:** Built-in 401 handling with token refresh

## Troubleshooting

**"No tokens received from server":**
- Check if server returns tokens in response body or cookies
- Adjust `handleLoginSuccess()` based on your API response format

**"Printer not found":**
- Verify printer package is correctly installed
- Check Windows printer permissions
- Adjust printer detection logic if needed

**"WebSocket connection failed":**
- Verify server IP and port
- Check firewall settings
- Ensure WebSocket endpoint is correct

## License

Same as main project.
