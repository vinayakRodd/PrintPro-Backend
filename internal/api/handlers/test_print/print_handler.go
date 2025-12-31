package test_print

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"print-pro-backend/internal/middleware/auth_middleware"
	"strings"
	"bytes"
)

// PrintHandler handles print requests from partners
type PrintHandler struct {
	uploadDir string
}

// NewPrintHandler creates a new print handler
func NewPrintHandler(uploadDir string) *PrintHandler {
	return &PrintHandler{
		uploadDir: uploadDir,
	}
}

// PrintRequest represents the request to print a file
type PrintRequest struct {
	Filename string `json:"filename"`
	Printer  string `json:"printer,omitempty"` // Optional: specific printer name (if not provided, uses default)
}

// PrintFile handles print requests from partners - sends file directly to printer
func (h *PrintHandler) PrintFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.sendErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed", "Only POST method is allowed")
		return
	}

	// Get user from context (set by auth middleware)
	user, ok := auth_middleware.GetUserFromContext(r)
	if !ok {
		h.sendErrorResponse(w, http.StatusUnauthorized, "Unauthorized", "User not found in session")
		return
	}

	// Verify user is a partner
	if user.UserType != "partner" {
		h.sendErrorResponse(w, http.StatusForbidden, "Forbidden", "Only partners can print files")
		return
	}

	// Parse request body
	var req PrintRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("ERROR: Failed to decode request body - %v", err)
		h.sendErrorResponse(w, http.StatusBadRequest, "Invalid request body", "Failed to parse JSON: "+err.Error())
		return
	}

	// Validate filename
	if req.Filename == "" {
		h.sendErrorResponse(w, http.StatusBadRequest, "Filename required", "Please provide a filename")
		return
	}

	// Sanitize filename (prevent path traversal)
	filename := filepath.Base(req.Filename) // Get only the base name
	filename = strings.ReplaceAll(filename, "..", "_")
	filename = strings.ReplaceAll(filename, "/", "_")
	filename = strings.ReplaceAll(filename, "\\", "_")

	// Build file path
	filePath := filepath.Join(h.uploadDir, filename)

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		log.Printf("ERROR: File not found - %s", filename)
		h.sendErrorResponse(w, http.StatusNotFound, "File not found", fmt.Sprintf("File '%s' does not exist", filename))
		return
	}

	// Get absolute file path
	absFilePath, err := filepath.Abs(filePath)
	if err != nil {
		log.Printf("ERROR: Failed to get absolute path - %v", err)
		h.sendErrorResponse(w, http.StatusInternalServerError, "Internal error", "Failed to resolve file path")
		return
	}

	// Print to printer using Windows native method
	if isWindows() {
		err = h.printWindowsFile(absFilePath, req.Printer)
	} else {
		err = h.printUnixFile(absFilePath, req.Printer)
	}

	if err != nil {
		log.Printf("ERROR: Failed to print file - %v", err)
		h.sendErrorResponse(w, http.StatusInternalServerError, "Print failed", err.Error())
		return
	}

	log.Printf("SUCCESS: File printed - Partner: %s, File: %s, Printer: %s", user.ID, filename, req.Printer)

	// Return success response
	h.sendJSONResponse(w, http.StatusOK, map[string]interface{}{
		"success":  true,
		"message":  "File sent to printer successfully",
		"filename": filename,
		"printer":  req.Printer,
	})
}

// ListFiles lists all available files for printing (for partners)
func (h *PrintHandler) ListFiles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.sendErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed", "Only GET method is allowed")
		return
	}

	// Get user from context
	user, ok := auth_middleware.GetUserFromContext(r)
	if !ok {
		h.sendErrorResponse(w, http.StatusUnauthorized, "Unauthorized", "User not found in session")
		return
	}

	// Verify user is a partner
	if user.UserType != "partner" {
		h.sendErrorResponse(w, http.StatusForbidden, "Forbidden", "Only partners can list files")
		return
	}

	// Read directory
	files, err := os.ReadDir(h.uploadDir)
	if err != nil {
		log.Printf("ERROR: Failed to read upload directory - %v", err)
		h.sendErrorResponse(w, http.StatusInternalServerError, "Failed to list files", err.Error())
		return
	}

	// Get file info
	fileList := []map[string]interface{}{}
	for _, file := range files {
		if file.IsDir() {
			continue
		}

		info, err := file.Info()
		if err != nil {
			continue
		}

		fileList = append(fileList, map[string]interface{}{
			"filename":    file.Name(),
			"size":        info.Size(),
			"modified_at": info.ModTime().Format("2006-01-02T15:04:05Z07:00"),
		})
	}

	log.Printf("SUCCESS: Files listed - Partner: %s, Count: %d", user.ID, len(fileList))

	h.sendJSONResponse(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"files":   fileList,
		"count":   len(fileList),
	})
}

// ListPrinters lists all available printers on the system
func (h *PrintHandler) ListPrinters(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.sendErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed", "Only GET method is allowed")
		return
	}

	// Get user from context
	user, ok := auth_middleware.GetUserFromContext(r)
	if !ok {
		h.sendErrorResponse(w, http.StatusUnauthorized, "Unauthorized", "User not found in session")
		return
	}

	// Verify user is a partner
	if user.UserType != "partner" {
		h.sendErrorResponse(w, http.StatusForbidden, "Forbidden", "Only partners can list printers")
		return
	}

	// Get list of printers based on OS
	var printers []map[string]interface{}
	var err error

	if isWindows() {
		printers, err = h.getWindowsPrinters()
	} else {
		printers, err = h.getUnixPrinters()
	}

	if err != nil {
		log.Printf("ERROR: Failed to get printers - %v", err)
		h.sendErrorResponse(w, http.StatusInternalServerError, "Failed to list printers", err.Error())
		return
	}

	log.Printf("SUCCESS: Printers listed - Partner: %s, Count: %d", user.ID, len(printers))

	h.sendJSONResponse(w, http.StatusOK, map[string]interface{}{
		"success":  true,
		"printers": printers,
		"count":    len(printers),
	})
}

// getWindowsPrinters gets list of printers on Windows
func (h *PrintHandler) getWindowsPrinters() ([]map[string]interface{}, error) {
	// Try PowerShell first
	psPaths := []string{
		"powershell",
		`C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe`,
		`C:\Windows\SysWOW64\WindowsPowerShell\v1.0\powershell.exe`,
	}

	var psPath string
	for _, path := range psPaths {
		if testCmd := exec.Command(path, "-Command", "exit 0"); testCmd.Run() == nil {
			psPath = path
			break
		}
	}

	if psPath != "" {
		psCommand := `Get-Printer | Select-Object Name, PrinterStatus, DriverName, PortName, Default | ConvertTo-Json`

		cmd := exec.Command(psPath, "-Command", psCommand)
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		err := cmd.Run()
		if err == nil {
			output := stdout.String()
			if strings.TrimSpace(output) != "" {
				var printerList []struct {
					Name         string  `json:"Name"`
					PrinterStatus *int   `json:"PrinterStatus"`
					DriverName   string  `json:"DriverName"`
					PortName     string  `json:"PortName"`
					Default      *bool   `json:"Default"`
				}

				if err := json.Unmarshal([]byte(output), &printerList); err == nil && len(printerList) > 0 {
					return h.processPrinterList(printerList)
				}
			}
		}
	}

	return []map[string]interface{}{}, fmt.Errorf("failed to get printers")
}

// processPrinterList processes printer list and filters virtual printers
func (h *PrintHandler) processPrinterList(printerList []struct {
	Name         string  `json:"Name"`
	PrinterStatus *int   `json:"PrinterStatus"`
	DriverName   string  `json:"DriverName"`
	PortName     string  `json:"PortName"`
	Default      *bool   `json:"Default"`
}) ([]map[string]interface{}, error) {
	virtualKeywords := []string{"pdf", "onenote", "fax", "microsoft", "document writer", "root print queue", "xps", "send to", "print to"}

	printers := make([]map[string]interface{}, 0)
	for _, p := range printerList {
		nameLower := strings.ToLower(p.Name)

		// Skip virtual printers
		isVirtual := false
		for _, keyword := range virtualKeywords {
			if strings.Contains(nameLower, keyword) {
				isVirtual = true
				break
			}
		}

		if isVirtual {
			continue
		}

		status := "online"
		if p.PrinterStatus != nil {
			statusCode := *p.PrinterStatus
			if statusCode == 6 || statusCode == 7 {
				status = "offline"
			}
		}

		isDefault := false
		if p.Default != nil && *p.Default {
			isDefault = true
		}

		printerInfo := map[string]interface{}{
			"name":       p.Name,
			"status":     status,
			"is_default": isDefault,
		}

		if p.DriverName != "" {
			printerInfo["driver"] = p.DriverName
		}
		if p.PortName != "" {
			printerInfo["port"] = p.PortName
		}

		printers = append(printers, printerInfo)
	}

	return printers, nil
}

// getUnixPrinters gets list of printers on Linux/Mac
func (h *PrintHandler) getUnixPrinters() ([]map[string]interface{}, error) {
	cmd := exec.Command("lpstat", "-p", "-d")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("failed to get printers: %v, stderr: %s", err, stderr.String())
	}

	output := stdout.String()
	lines := strings.Split(output, "\n")

	printers := make([]map[string]interface{}, 0)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "printer ") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				printerName := parts[1]
				status := "unknown"
				if len(parts) >= 3 {
					status = strings.ToLower(strings.TrimSuffix(parts[2], "."))
				}

				printers = append(printers, map[string]interface{}{
					"name":       printerName,
					"status":     status,
					"is_default": false,
				})
			}
		}
	}

	return printers, nil
}

// printWindowsFile prints a file on Windows directly to the specified printer
func (h *PrintHandler) printWindowsFile(filePath, printerName string) error {
	// Find PowerShell path
	psPaths := []string{
		`C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe`,
		`C:\Windows\SysWOW64\WindowsPowerShell\v1.0\powershell.exe`,
		"powershell",
	}

	var psPath string
	for _, path := range psPaths {
		if testCmd := exec.Command(path, "-Command", "exit 0"); testCmd.Run() == nil {
			psPath = path
			break
		}
	}

	if psPath == "" {
		return fmt.Errorf("PowerShell not found, cannot print")
	}

	// PowerShell script to print directly to printer WITHOUT opening browser/application
	// This uses Windows print spooler API to send file directly to printer
	psCommand := fmt.Sprintf(`
		$printerName = "%s"
		$filePath = "%s"
		
		# Get current default printer
		$currentDefault = (Get-WmiObject -Class Win32_Printer | Where-Object {$_.Default -eq $true}).Name
		
		# Set specified printer as default
		$targetPrinter = Get-WmiObject -Class Win32_Printer -Filter "Name='$printerName'"
		if (-not $targetPrinter) {
			Write-Error "Printer '$printerName' not found"
			exit 1
		}
		
		$targetPrinter.SetDefaultPrinter()
		Write-Host "Set printer '$printerName' as default"
		
		# Method: Use Start-Process with print verb, then automatically close opened windows
		# This sends file to default printer and closes any application windows that open
		try {
			# Start print process (may open application window like Edge)
			$psi = New-Object System.Diagnostics.ProcessStartInfo
			$psi.FileName = $filePath
			$psi.Verb = "print"
			$psi.UseShellExecute = $true
			$psi.WindowStyle = [System.Diagnostics.ProcessWindowStyle]::Minimized
			$psi.CreateNoWindow = $false
			
			$process = [System.Diagnostics.Process]::Start($psi)
			
			if ($process) {
				Write-Host "Print process started (PID: $($process.Id))"
				
				# Wait for print job to be queued (application may open)
				Start-Sleep -Milliseconds 2000
				
				# Automatically close the window if it's still open
				if (-not $process.HasExited) {
					# Try to close the main window gracefully
					$closed = $process.CloseMainWindow()
					Start-Sleep -Milliseconds 1000
					
					# If still running, force kill it
					if (-not $process.HasExited) {
						$process.Kill()
						Write-Host "Closed application window"
					}
				}
				
				Write-Host "Print job sent to printer"
			} else {
				Write-Error "Failed to start print process"
				exit 1
			}
			
			# Verify print job was queued
			$printJobs = Get-WmiObject -Class Win32_PrintJob | Where-Object { $_.PrinterName -eq $printerName }
			if ($printJobs) {
				$jobCount = ($printJobs | Measure-Object).Count
				Write-Host "SUCCESS: Print job queued - Found $jobCount job(s)"
			} else {
				Write-Host "INFO: Print job sent to printer (may have printed immediately)"
			}
		} catch {
			Write-Error "Failed to print: $_"
			exit 1
		} finally {
			# Restore original default printer
			if ($currentDefault -and $currentDefault -ne $printerName) {
				$originalPrinter = Get-WmiObject -Class Win32_Printer -Filter "Name='$currentDefault'"
				if ($originalPrinter) {
					$originalPrinter.SetDefaultPrinter()
					Write-Host "Restored default printer to '$currentDefault'"
				}
			}
		}
	`, printerName, filePath)

	cmd := exec.Command(psPath, "-Command", psCommand)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Stdout = nil

	err := cmd.Run()
	if err != nil {
		errorOutput := stderr.String()
		log.Printf("DEBUG: PowerShell print command error: %s", errorOutput)
		
		// Check if it's a "no application associated" error
		if strings.Contains(errorOutput, "No application is associated") {
			// Try using Microsoft Edge directly
			return h.printWindowsFileWithEdge(filePath, printerName)
		}
		
		return fmt.Errorf("print command failed: %v, output: %s", err, errorOutput)
	}

	return nil
}

// printWindowsFileWithEdge prints using Microsoft Edge (fallback when file associations are missing)
func (h *PrintHandler) printWindowsFileWithEdge(filePath, printerName string) error {
	psPaths := []string{
		`C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe`,
		`C:\Windows\SysWOW64\WindowsPowerShell\v1.0\powershell.exe`,
		"powershell",
	}

	var psPath string
	for _, path := range psPaths {
		if testCmd := exec.Command(path, "-Command", "exit 0"); testCmd.Run() == nil {
			psPath = path
			break
		}
	}

	if psPath == "" {
		return fmt.Errorf("PowerShell not found")
	}

	edgePaths := []string{
		`C:\Program Files\Microsoft\Edge\Application\msedge.exe`,
		`C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe`,
	}

	var edgePath string
	for _, path := range edgePaths {
		if _, err := os.Stat(path); err == nil {
			edgePath = path
			break
		}
	}

	if edgePath == "" {
		return fmt.Errorf("Microsoft Edge not found")
	}

	psCommand := fmt.Sprintf(`
		$printerName = "%s"
		$filePath = "%s"
		$edgePath = "%s"
		
		$currentDefault = (Get-WmiObject -Class Win32_Printer | Where-Object {$_.Default -eq $true}).Name
		$targetPrinter = Get-WmiObject -Class Win32_Printer -Filter "Name='$printerName'"
		if (-not $targetPrinter) {
			Write-Error "Printer '$printerName' not found"
			exit 1
		}
		
		$targetPrinter.SetDefaultPrinter()
		
		try {
			$psi = New-Object System.Diagnostics.ProcessStartInfo
			$psi.FileName = $edgePath
			$psi.Arguments = '"' + $filePath + '" --print'
			$psi.UseShellExecute = $false
			$psi.WindowStyle = [System.Diagnostics.ProcessWindowStyle]::Hidden
			$psi.CreateNoWindow = $true
			
			$process = [System.Diagnostics.Process]::Start($psi)
			if ($process) {
				Start-Sleep -Milliseconds 4000
				Write-Host "Print job sent via Edge"
			}
		} catch {
			Write-Error "Failed to print with Edge: $_"
			exit 1
		} finally {
			if ($currentDefault -and $currentDefault -ne $printerName) {
				$originalPrinter = Get-WmiObject -Class Win32_Printer -Filter "Name='$currentDefault'"
				if ($originalPrinter) {
					$originalPrinter.SetDefaultPrinter()
				}
			}
		}
	`, printerName, filePath, edgePath)

	cmd := exec.Command(psPath, "-Command", psCommand)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Stdout = nil

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("Edge print failed: %v, output: %s", err, stderr.String())
	}

	return nil
}

// printUnixFile prints a file on Linux/Mac
func (h *PrintHandler) printUnixFile(filePath, printerName string) error {
	var cmd *exec.Cmd
	if printerName != "" {
		cmd = exec.Command("lp", "-d", printerName, filePath)
	} else {
		cmd = exec.Command("lp", filePath)
	}

	cmd.Stdout = nil
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("lp command failed: %v, stderr: %s", err, stderr.String())
	}

	return nil
}

// isWindows checks if the system is Windows
func isWindows() bool {
	return os.PathSeparator == '\\' || strings.Contains(strings.ToLower(os.Getenv("OS")), "windows")
}

// sendJSONResponse sends a JSON response
func (h *PrintHandler) sendJSONResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("ERROR: Failed to encode JSON response - %v", err)
	}
}

// sendErrorResponse sends an error JSON response
func (h *PrintHandler) sendErrorResponse(w http.ResponseWriter, statusCode int, message, error string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": false,
		"message": message,
		"error":   error,
	})
}
