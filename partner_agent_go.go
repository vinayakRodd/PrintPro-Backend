package main

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"github.com/alexbrainman/printer"
	"github.com/gorilla/websocket"
	"golang.org/x/crypto/pbkdf2"
)

// ============================================================================
// CONFIGURATION & TYPES
// ============================================================================

const (
	defaultServerIP = "10.213.120.142"
	serverPort      = "8080"
	tokenFile       = "tokens.enc"
	blockList       = "MICROSOFT,ONENOTE,PDF,FAX,XPS,SEND TO,ROOT"
)

type TokenData struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
}

type PrintJobParams struct {
	Filename            string
	PrinterName         string
	Color               bool
	NumCopies           int
	StartPage           *int
	EndPage             *int
	PageFilterType      string
	IndividualColorPages []int
	SkipPages           []int
	BackToBack          bool
	PDFData             []byte
}

type PrintAgent struct {
	app            fyne.App
	window         fyne.Window
	serverIP       *widget.Entry
	emailEntry     *widget.Entry
	passwordEntry  *widget.Entry
	otpEntry       *widget.Entry
	statusLabel    *widget.Label
	logArea        *widget.Entry
	connectBtn     *widget.Button
	loginBtn       *widget.Button
	verifyOTPBtn   *widget.Button
	
	// State
	isConnected    bool
	tokens         *TokenData
	tokenMutex     sync.RWMutex
	httpClient     *http.Client
	wsConn         *websocket.Conn
	wsMutex        sync.Mutex
	flipChan       chan bool
	logChan        chan string
	
	// Server URL
	baseURL        string
}

// ============================================================================
// TOKEN MANAGEMENT (ENCRYPTED STORAGE)
// ============================================================================

func getTokenFilePath() (string, error) {
	usr, err := user.Current()
	if err != nil {
		return tokenFile, nil // Fallback to current directory
	}
	tokenDir := filepath.Join(usr.HomeDir, ".print_agent")
	os.MkdirAll(tokenDir, 0700)
	return filepath.Join(tokenDir, tokenFile), nil
}

func deriveKey(password string, salt []byte) []byte {
	return pbkdf2.Key([]byte(password), salt, 4096, 32, sha256.New)
}

func encrypt(data []byte, password string) ([]byte, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return nil, err
	}
	
	key := deriveKey(password, salt)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	
	ciphertext := gcm.Seal(nonce, nonce, data, nil)
	result := append(salt, ciphertext...)
	return result, nil
}

func decrypt(data []byte, password string) ([]byte, error) {
	if len(data) < 16 {
		return nil, fmt.Errorf("invalid encrypted data")
	}
	
	salt := data[:16]
	ciphertext := data[16:]
	
	key := deriveKey(password, salt)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("invalid ciphertext")
	}
	
	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	return gcm.Open(nil, nonce, ciphertext, nil)
}

func (p *PrintAgent) saveTokens() error {
	p.tokenMutex.RLock()
	defer p.tokenMutex.RUnlock()
	
	if p.tokens == nil {
		return nil
	}
	
	data, err := json.Marshal(p.tokens)
	if err != nil {
		return err
	}
	
	// Use machine-specific key (derived from hostname)
	hostname, _ := os.Hostname()
	password := hostname + "print-agent-secret-key"
	
	encrypted, err := encrypt(data, password)
	if err != nil {
		return err
	}
	
	tokenPath, err := getTokenFilePath()
	if err != nil {
		return err
	}
	
	return ioutil.WriteFile(tokenPath, encrypted, 0600)
}

func (p *PrintAgent) loadTokens() error {
	tokenPath, err := getTokenFilePath()
	if err != nil {
		return err
	}
	
	data, err := ioutil.ReadFile(tokenPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No tokens yet
		}
		return err
	}
	
	hostname, _ := os.Hostname()
	password := hostname + "print-agent-secret-key"
	
	decrypted, err := decrypt(data, password)
	if err != nil {
		return err
	}
	
	var tokens TokenData
	if err := json.Unmarshal(decrypted, &tokens); err != nil {
		return err
	}
	
	// Check if refresh token is still valid
	if time.Now().After(tokens.ExpiresAt) {
		os.Remove(tokenPath) // Delete expired tokens
		return nil
	}
	
	p.tokenMutex.Lock()
	p.tokens = &tokens
	p.tokenMutex.Unlock()
	
	return nil
}

// ============================================================================
// AUTHENTICATION FLOW
// ============================================================================

func (p *PrintAgent) login() {
	email := p.emailEntry.Text
	password := p.passwordEntry.Text
	
	if email == "" || password == "" {
		p.writeLog("❌ Please enter email and password")
		return
	}
	
	p.writeLog(fmt.Sprintf("🔐 Logging in as %s...", email))
	p.loginBtn.SetText("Logging in...")
	p.loginBtn.Disable()
	
	go func() {
		reqBody := map[string]string{
			"email":    email,
			"password": password,
		}
		
		jsonData, _ := json.Marshal(reqBody)
		resp, err := p.httpClient.Post(
			fmt.Sprintf("%s/api/auth/login/partner", p.baseURL),
			"application/json",
			bytes.NewBuffer(jsonData),
		)
		
		p.app.Exec(func() {
			p.loginBtn.SetText("Login")
			p.loginBtn.Enable()
		})
		
		if err != nil {
			p.writeLog(fmt.Sprintf("❌ Login error: %v", err))
			return
		}
		defer resp.Body.Close()
		
		if resp.StatusCode != http.StatusOK {
			body, _ := ioutil.ReadAll(resp.Body)
			p.writeLog(fmt.Sprintf("❌ Login failed: %s", string(body)))
			return
		}
		
		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			p.writeLog(fmt.Sprintf("❌ Failed to parse response: %v", err))
			return
		}
		
		// Check if OTP is required
		if result["otp_required"] == true {
			p.app.Exec(func() {
				p.writeLog("✅ OTP sent to your email. Please enter it below.")
				p.otpEntry.Show()
				p.verifyOTPBtn.Show()
			})
		} else {
			// Direct login (no OTP)
			p.handleLoginSuccess(result)
		}
	}()
}

func (p *PrintAgent) verifyOTP() {
	email := p.emailEntry.Text
	otp := p.otpEntry.Text
	
	if email == "" || otp == "" {
		p.writeLog("❌ Please enter email and OTP")
		return
	}
	
	p.writeLog("🔐 Verifying OTP...")
	p.verifyOTPBtn.SetText("Verifying...")
	p.verifyOTPBtn.Disable()
	
	go func() {
		reqBody := map[string]string{
			"email": email,
			"otp":   otp,
		}
		
		jsonData, _ := json.Marshal(reqBody)
		resp, err := p.httpClient.Post(
			fmt.Sprintf("%s/api/auth/otp/verify", p.baseURL),
			"application/json",
			bytes.NewBuffer(jsonData),
		)
		
		p.app.Exec(func() {
			p.verifyOTPBtn.SetText("Verify OTP")
			p.verifyOTPBtn.Enable()
		})
		
		if err != nil {
			p.writeLog(fmt.Sprintf("❌ OTP verification error: %v", err))
			return
		}
		defer resp.Body.Close()
		
		if resp.StatusCode != http.StatusOK {
			body, _ := ioutil.ReadAll(resp.Body)
			p.writeLog(fmt.Sprintf("❌ OTP verification failed: %s", string(body)))
			return
		}
		
		// After OTP verification, get tokens via refresh endpoint
		// (Assuming the server sets a refresh token cookie or returns it)
		// For now, we'll call a token endpoint or use the refresh token from login
		p.writeLog("✅ OTP verified! Getting tokens...")
		
		// Try to get tokens - this might need adjustment based on your API
		// For now, assume we need to call refresh with the session
		p.refreshAccessToken()
	}()
}

func (p *PrintAgent) handleLoginSuccess(result map[string]interface{}) {
	// Extract tokens from response
	// Adjust based on your API response structure
	accessToken, _ := result["access_token"].(string)
	refreshToken, _ := result["refresh_token"].(string)
	
	// Also check cookies for refresh token
	cookies := p.httpClient.Jar.Cookies(&url.URL{Scheme: "http", Host: p.baseURL})
	for _, cookie := range cookies {
		if cookie.Name == "refresh_token" {
			refreshToken = cookie.Value
		}
	}
	
	if accessToken == "" || refreshToken == "" {
		p.writeLog("❌ No tokens received from server")
		return
	}
	
	p.tokenMutex.Lock()
	p.tokens = &TokenData{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    time.Now().Add(7 * 24 * time.Hour), // 7 days
	}
	p.tokenMutex.Unlock()
	
	p.saveTokens()
	p.writeLog("✅ Login successful! Tokens saved.")
	
	p.app.Exec(func() {
		p.statusLabel.SetText("● AUTHENTICATED")
		p.connectBtn.Enable()
	})
}

func (p *PrintAgent) refreshAccessToken() bool {
	p.tokenMutex.RLock()
	refreshToken := p.tokens.RefreshToken
	p.tokenMutex.RUnlock()
	
	if refreshToken == "" {
		return false
	}
	
	// Use Authorization header for refresh token (more reliable for Go agent)
	req, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/auth/refresh", p.baseURL), nil)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", refreshToken))
	
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return false
	}
	
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false
	}
	
	accessToken, _ := result["access_token"].(string)
	if accessToken == "" {
		return false
	}
	
	p.tokenMutex.Lock()
	p.tokens.AccessToken = accessToken
	p.tokenMutex.Unlock()
	
	p.saveTokens()
	return true
}

func (p *PrintAgent) getAuthHeader() string {
	p.tokenMutex.RLock()
	defer p.tokenMutex.RUnlock()
	
	if p.tokens == nil || p.tokens.AccessToken == "" {
		return ""
	}
	
	return "Bearer " + p.tokens.AccessToken
}

func (p *PrintAgent) makeAuthenticatedRequest(method, url string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}
	
	authHeader := p.getAuthHeader()
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	
	// Auto-refresh on 401
	if resp.StatusCode == http.StatusUnauthorized {
		if p.refreshAccessToken() {
			// Retry request
			req.Header.Set("Authorization", p.getAuthHeader())
			resp.Body.Close()
			return p.httpClient.Do(req)
		} else {
			// Refresh failed - need to re-login
			p.app.Exec(func() {
				p.writeLog("⚠️ Session expired. Please login again.")
				p.statusLabel.SetText("● OFFLINE")
				p.tokens = nil
				p.saveTokens()
			})
		}
	}
	
	return resp, nil
}

// ============================================================================
// PRINTER DETECTION
// ============================================================================

func (p *PrintAgent) getRealtimePhysicalPrinters() []string {
	var onlinePrinters []string
	blockListUpper := strings.ToUpper(blockList)
	blocked := strings.Split(blockListUpper, ",")
	
	// Get all printers
	printers, err := printer.ReadNames()
	if err != nil {
		p.writeLog(fmt.Sprintf("❌ Hardware Detection Error: %v", err))
		return onlinePrinters
	}
	
	for _, printerName := range printers {
		// Check block list
		skip := false
		for _, blocked := range blocked {
			if strings.Contains(strings.ToUpper(printerName), strings.TrimSpace(blocked)) {
				skip = true
				break
			}
		}
		if skip {
			continue
		}
		
		// Check if printer is online
		handle, err := printer.Open(printerName)
		if err != nil {
			continue
		}
		
		info, err := handle.GetJob(0)
		handle.Close()
		
		// Simple check - if we can open it, consider it online
		// You may need to adjust this based on the printer package API
		if err == nil || strings.Contains(err.Error(), "job") {
			onlinePrinters = append(onlinePrinters, printerName)
		}
	}
	
	return onlinePrinters
}

func (p *PrintAgent) findPrinter() string {
	printers := p.getRealtimePhysicalPrinters()
	for _, name := range printers {
		if strings.Contains(strings.ToUpper(name), "L3210") {
			return name
		}
	}
	
	// Return default printer
	defaultPrinter, _ := printer.Default()
	return defaultPrinter
}

// ============================================================================
// NETWORKING (PRINTER SYNC & WEBSOCKET)
// ============================================================================

func (p *PrintAgent) printerSyncLoop() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	
	for p.isConnected {
		select {
		case <-ticker.C:
			printers := p.getRealtimePhysicalPrinters()
			if len(printers) == 0 {
				p.writeLog("🔍 No physical printers detected")
				continue
			}
			
			payload := map[string]interface{}{
				"printers": printers,
			}
			
			jsonData, _ := json.Marshal(payload)
			req, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/partner-agent/sync-printers", p.baseURL), bytes.NewBuffer(jsonData))
			req.Header.Set("Content-Type", "application/json")
			authHeader := p.getAuthHeader()
			if authHeader != "" {
				req.Header.Set("Authorization", authHeader)
			}
			
			resp, err := p.httpClient.Do(req)
			if err != nil {
				p.writeLog(fmt.Sprintf("⚠️ Sync error: %v", err))
				continue
			}
			resp.Body.Close()
			
			if resp.StatusCode == http.StatusOK {
				p.writeLog(fmt.Sprintf("✅ Printers synced: %v", printers))
			}
		}
	}
}

func (p *PrintAgent) websocketLoop() {
	for p.isConnected {
		wsURL := fmt.Sprintf("ws://%s:%s/ws/printer1", p.serverIP.Text, serverPort)
		
		dialer := websocket.Dialer{}
		conn, _, err := dialer.Dial(wsURL, nil)
		if err != nil {
			p.writeLog(fmt.Sprintf("⚠️ WebSocket Error: %v", err))
			time.Sleep(5 * time.Second)
			continue
		}
		
		p.wsMutex.Lock()
		p.wsConn = conn
		p.wsMutex.Unlock()
		
		p.app.Exec(func() {
			p.statusLabel.SetText("● ONLINE")
		})
		
		// Read messages
		for {
			var msg map[string]interface{}
			if err := conn.ReadJSON(&msg); err != nil {
				p.writeLog(fmt.Sprintf("⚠️ WebSocket read error: %v", err))
				break
			}
			
			if action, ok := msg["action"].(string); ok && action == "print_job_available" {
				p.writeLog("📩 Job detected. Downloading...")
				go p.handlePrintJob()
			}
		}
		
		p.wsMutex.Lock()
		p.wsConn = nil
		p.wsMutex.Unlock()
		
		p.app.Exec(func() {
			p.statusLabel.SetText("● OFFLINE")
		})
		
		if p.isConnected {
			time.Sleep(5 * time.Second)
		}
	}
}

// ============================================================================
// PRINT JOB HANDLING
// ============================================================================

func (p *PrintAgent) handlePrintJob() {
	req, _ := http.NewRequest("GET", fmt.Sprintf("%s/api/partner-agent/fetch-job", p.baseURL), nil)
	authHeader := p.getAuthHeader()
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	
	resp, err := p.makeAuthenticatedRequest("GET", fmt.Sprintf("%s/api/partner-agent/fetch-job", p.baseURL), nil)
	if err != nil {
		p.writeLog(fmt.Sprintf("⚠️ Job Error: %v", err))
		return
	}
	defer resp.Body.Close()
	
	if resp.StatusCode == http.StatusNoContent {
		p.writeLog("ℹ️ No jobs available")
		return
	}
	
	if resp.StatusCode != http.StatusOK {
		p.writeLog(fmt.Sprintf("❌ Failed to fetch job: %d", resp.StatusCode))
		return
	}
	
	// Parse headers
	params := PrintJobParams{
		Filename:       resp.Header.Get("X-File-Name"),
		PrinterName:    resp.Header.Get("X-Print-Printer-Name"),
		Color:          resp.Header.Get("X-Print-Color") == "true",
		NumCopies:      1,
		PageFilterType: resp.Header.Get("X-Print-Page-Filter"),
		BackToBack:     resp.Header.Get("X-Print-Back-To-Back") == "true",
	}
	
	// Parse copies
	if copiesStr := resp.Header.Get("X-Print-Copies"); copiesStr != "" {
		if copies, err := strconv.Atoi(copiesStr); err == nil && copies > 0 {
			params.NumCopies = copies
		}
	}
	
	// Parse page range
	if startStr := resp.Header.Get("X-Print-Start-Page"); startStr != "" {
		if start, err := strconv.Atoi(startStr); err == nil && start > 0 {
			params.StartPage = &start
		}
	}
	if endStr := resp.Header.Get("X-Print-End-Page"); endStr != "" {
		if end, err := strconv.Atoi(endStr); err == nil && end > 0 {
			params.EndPage = &end
		}
	}
	
	// Parse JSON arrays
	if colorPagesStr := resp.Header.Get("X-Print-Individual-Color-Pages"); colorPagesStr != "" {
		json.Unmarshal([]byte(colorPagesStr), &params.IndividualColorPages)
	}
	if skipPagesStr := resp.Header.Get("X-Print-Skip-Pages"); skipPagesStr != "" {
		json.Unmarshal([]byte(skipPagesStr), &params.SkipPages)
	}
	
	// Read PDF data
	pdfData, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		p.writeLog(fmt.Sprintf("❌ Failed to read PDF: %v", err))
		return
	}
	params.PDFData = pdfData
	
	// Log job details
	p.writeLog(fmt.Sprintf("📦 Job: %s", params.Filename))
	if params.PrinterName != "" {
		p.writeLog(fmt.Sprintf("   Printer: %s (server-selected)", params.PrinterName))
	}
	p.writeLog(fmt.Sprintf("   Color: %v, Copies: %d", params.Color, params.NumCopies))
	
	// Print the job
	if p.printJob(params) {
		// Confirm print
		confirmData := map[string]string{
			"filename": params.Filename,
			"status":   "completed",
		}
		jsonData, _ := json.Marshal(confirmData)
		req, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/partner-agent/confirm-print", p.baseURL), bytes.NewBuffer(jsonData))
		req.Header.Set("Content-Type", "application/json")
		authHeader := p.getAuthHeader()
		if authHeader != "" {
			req.Header.Set("Authorization", authHeader)
		}
		p.httpClient.Do(req)
		p.writeLog("✅ Job Confirmed.")
	} else {
		p.writeLog("❌ Print Failed.")
	}
}

// ============================================================================
// PDF PRINTING (SIMPLIFIED - YOU'LL NEED A PDF LIBRARY)
// ============================================================================

func (p *PrintAgent) printJob(params PrintJobParams) bool {
	// NOTE: This is a simplified version. You'll need to:
	// 1. Use a PDF library like github.com/gen2brain/go-fitz or github.com/pdfcpu/pdfcpu
	// 2. Convert PDF pages to images
	// 3. Use Windows GDI+ or similar to print images
	// 4. Handle color/grayscale conversion
	// 5. Implement page filtering, skip pages, etc.
	
	// For now, this is a placeholder that shows the structure
	p.writeLog(fmt.Sprintf("🖨️ Printing to: %s", params.PrinterName))
	
	// Validate printer
	if params.PrinterName == "" {
		params.PrinterName = p.findPrinter()
	}
	
	availablePrinters := p.getRealtimePhysicalPrinters()
	printerFound := false
	for _, name := range availablePrinters {
		if name == params.PrinterName {
			printerFound = true
			break
		}
	}
	
	if !printerFound {
		p.writeLog(fmt.Sprintf("⚠️ Printer '%s' not available, using default", params.PrinterName))
		params.PrinterName = p.findPrinter()
	}
	
	// TODO: Implement actual PDF printing here
	// This requires:
	// - PDF parsing library
	// - Image rendering
	// - Windows printing API
	// - Color/grayscale conversion
	// - Page filtering logic
	// - Manual duplex with Fyne dialog
	
	// Manual duplex example (conceptual):
	if params.BackToBack {
		// Print odd pages first
		p.writeLog("📄 Printing Front Pages (Odds)...")
		// ... print odd pages ...
		
		// Show flip dialog
		p.app.Exec(func() {
			dialog.ShowConfirm(
				"⚠️ FLIP PAPER - Manual Duplex Required",
				"FRONT SIDE PRINTED ✅\n\nPlease flip the pages and click YES to continue.",
				func(confirmed bool) {
					p.flipChan <- confirmed
				},
				p.window,
			)
		})
		
		// Wait for user confirmation
		confirmed := <-p.flipChan
		if confirmed {
			p.writeLog("📄 Printing Back Pages (Evens)...")
			// ... print even pages ...
		} else {
			p.writeLog("⚠️ Flip cancelled - skipping back pages")
		}
	} else {
		// Simplex printing
		// ... print all pages ...
	}
	
	return true
}

// ============================================================================
// UI SETUP
// ============================================================================

func (p *PrintAgent) writeLog(message string) {
	timestamp := time.Now().Format("15:04:05")
	logMsg := fmt.Sprintf("[%s] %s\n", timestamp, message)
	
	select {
	case p.logChan <- logMsg:
	default:
	}
	
	p.app.Exec(func() {
		currentText := p.logArea.Text
		p.logArea.SetText(currentText + logMsg)
		p.logArea.CursorRow = len(strings.Split(p.logArea.Text, "\n"))
	})
}

func (p *PrintAgent) startNetworking() {
	if p.isConnected {
		return
	}
	
	// Check authentication
	p.tokenMutex.RLock()
	hasTokens := p.tokens != nil && p.tokens.AccessToken != ""
	p.tokenMutex.RUnlock()
	
	if !hasTokens {
		p.writeLog("❌ Please login first")
		return
	}
	
	p.isConnected = true
	p.baseURL = fmt.Sprintf("http://%s:%s", p.serverIP.Text, serverPort)
	
	p.serverIP.Disable()
	p.connectBtn.SetText("RUNNING...")
	p.connectBtn.Disable()
	
	p.writeLog(fmt.Sprintf("🚀 Initializing connection using IP: %s...", p.serverIP.Text))
	
	go p.printerSyncLoop()
	go p.websocketLoop()
}

func (p *PrintAgent) setupUI() {
	p.window = p.app.NewWindow("Cloud Print Agent - Multi-Printer Support (Go)")
	p.window.Resize(fyne.NewSize(700, 550))
	
	// Server IP
	p.serverIP = widget.NewEntry()
	p.serverIP.SetText(defaultServerIP)
	
	// Login fields
	p.emailEntry = widget.NewEntry()
	p.emailEntry.SetPlaceHolder("Email")
	
	p.passwordEntry = widget.NewPasswordEntry()
	p.passwordEntry.SetPlaceHolder("Password")
	
	p.otpEntry = widget.NewEntry()
	p.otpEntry.SetPlaceHolder("OTP")
	p.otpEntry.Hide()
	
	// Buttons
	p.loginBtn = widget.NewButton("Login", p.login)
	p.verifyOTPBtn = widget.NewButton("Verify OTP", p.verifyOTP)
	p.verifyOTPBtn.Hide()
	
	p.connectBtn = widget.NewButton("START AGENT", p.startNetworking)
	p.connectBtn.Disable()
	
	// Status
	p.statusLabel = widget.NewLabel("● OFFLINE")
	p.statusLabel.Alignment = fyne.TextAlignTrailing
	
	// Log area
	p.logArea = widget.NewMultiLineEntry()
	p.logArea.SetReadOnly(true)
	
	// Layout
	topBar := container.NewHBox(
		widget.NewLabel("SERVER IP:"),
		p.serverIP,
		p.connectBtn,
		container.NewBorder(nil, nil, nil, p.statusLabel),
	)
	
	loginSection := container.NewVBox(
		widget.NewLabel("Authentication"),
		container.NewHBox(p.emailEntry, p.passwordEntry, p.loginBtn),
		container.NewHBox(p.otpEntry, p.verifyOTPBtn),
	)
	
	content := container.NewBorder(
		container.NewVBox(topBar, loginSection),
		nil,
		nil,
		nil,
		container.NewScroll(p.logArea),
	)
	
	p.window.SetContent(content)
}

// ============================================================================
// MAIN
// ============================================================================

func main() {
	agent := &PrintAgent{
		app:      app.NewWithID("com.printagent.app"),
		httpClient: &http.Client{Timeout: 20 * time.Second},
		flipChan: make(chan bool, 1),
		logChan:  make(chan string, 100),
	}
	
	// Load saved tokens
	agent.loadTokens()
	
	// Setup UI
	agent.setupUI()
	
	// Start log processor
	go func() {
		for logMsg := range agent.logChan {
			agent.app.Exec(func() {
				currentText := agent.logArea.Text
				agent.logArea.SetText(currentText + logMsg)
			})
		}
	}()
	
	// Show window
	agent.window.ShowAndRun()
}
