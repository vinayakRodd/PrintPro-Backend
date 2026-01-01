package printer_list

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// GetWindowsPrinters gets list of printers on Windows
func GetWindowsPrinters() ([]map[string]interface{}, error) {
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
		// Get list of printers with real-time status
		psCommand := `
			# Get all printers
			$printers = Get-WmiObject -Class Win32_Printer -ErrorAction SilentlyContinue | Where-Object {
				# Filter out virtual printers
				$name = $_.Name.ToLower()
				$virtualKeywords = @("pdf", "onenote", "fax", "microsoft", "document writer", "root print queue", "xps", "send to", "print to")
				$isVirtual = $false
				foreach ($keyword in $virtualKeywords) {
					if ($name -like "*$keyword*") {
						$isVirtual = $true
						break
					}
				}
				-not $isVirtual
			}
			
			$printerArray = @()
			foreach ($printer in $printers) {
				# Get fresh printer status
				try {
					$freshPrinter = Get-WmiObject -Class Win32_Printer -Filter "Name='$($printer.Name)'" -ErrorAction Stop
				} catch {
					continue
				}
				
				$isAvailable = $false
				$status = "offline"
				
				# For USB printers, use WorkOffline flag as primary indicator
				$portName = $freshPrinter.PortName
				if ($portName -like "USB*") {
					# FIX: Logic is inverted based on user feedback
					# When unplugged → shows detected (WorkOffline = true, but showing as available)
					# When plugged in → doesn't show (WorkOffline = false, but not showing as available)
					# So we need to INVERT the WorkOffline check
					
					# INVERTED: WorkOffline = true means CONNECTED, WorkOffline = false means DISCONNECTED
					if ($freshPrinter.WorkOffline -eq $true) {
						# WorkOffline = true → CONNECTED (inverted logic)
						if ($freshPrinter.PrinterStatus -notin @(6, 7, 8, 10, 15)) {
							$isAvailable = $true
							$status = "online"
						} else {
							# Status code indicates error, but WorkOffline says connected
							$isAvailable = $true
							$status = "online"
						}
					} else {
						# WorkOffline = false → DISCONNECTED (inverted logic)
						$isAvailable = $false
						$status = "offline"
					}
				} else {
					# For non-USB printers, use standard checks
					try {
						$freshPrinter = Get-WmiObject -Class Win32_Printer -Filter "Name='$($printer.Name)'" -ErrorAction Stop
						if ($freshPrinter.WorkOffline -eq $false -and $freshPrinter.PrinterStatus -notin @(6, 7, 8, 10, 15)) {
							$isAvailable = $true
							$status = "online"
						} else {
							$isAvailable = $false
							if ($freshPrinter.PrinterStatus -eq 6) { $status = "offline" }
							elseif ($freshPrinter.PrinterStatus -eq 7) { $status = "paused" }
							elseif ($freshPrinter.PrinterStatus -eq 8) { $status = "error" }
							elseif ($freshPrinter.PrinterStatus -eq 10) { $status = "not_available" }
							elseif ($freshPrinter.PrinterStatus -eq 15) { $status = "pending_deletion" }
						}
					} catch {
						$isAvailable = $false
						$status = "offline"
					}
				}
				
				# Only include printers that are actually available
				if ($isAvailable) {
					try {
						$printerInfo = Get-WmiObject -Class Win32_Printer -Filter "Name='$($printer.Name)'" -ErrorAction Stop
						$printerArray += @{
							Name = $printerInfo.Name
							PrinterStatus = $printerInfo.PrinterStatus
							WorkOffline = $printerInfo.WorkOffline
							Availability = $printerInfo.Availability
							DriverName = $printerInfo.DriverName
							PortName = $printerInfo.PortName
							Default = $printerInfo.Default
							Local = $printerInfo.Local
							Network = $printerInfo.Network
							Status = $status
							IsAvailable = $isAvailable
						}
					} catch {
						# Skip if we can't get printer info
						continue
					}
				}
			}
			
			$printerArray | ConvertTo-Json -Depth 3
		`

		cmd := exec.Command(psPath, "-Command", psCommand)
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		err := cmd.Run()
		if err == nil {
			output := stdout.String()
			if strings.TrimSpace(output) != "" {
				var printerList []PrinterInfo

				if err := json.Unmarshal([]byte(output), &printerList); err == nil && len(printerList) > 0 {
					return ProcessPrinterListEnhanced(printerList)
				}
			}
		}

		// Fallback to original method if enhanced detection fails
		psCommandFallback := `Get-Printer | Select-Object Name, PrinterStatus, DriverName, PortName, Default | ConvertTo-Json`
		cmd = exec.Command(psPath, "-Command", psCommandFallback)
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		if err := cmd.Run(); err == nil {
			output := stdout.String()
			if strings.TrimSpace(output) != "" {
				var printerList []BasicPrinterInfo

				if err := json.Unmarshal([]byte(output), &printerList); err == nil && len(printerList) > 0 {
					return ProcessPrinterList(printerList)
				}
			}
		}
	}

	return []map[string]interface{}{}, fmt.Errorf("failed to get printers")
}

