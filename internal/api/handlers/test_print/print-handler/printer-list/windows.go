package printer_list

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
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
				
				# Check printer availability using WorkOffline flag and PrinterStatus
				# Windows standard: WorkOffline = false means ONLINE, WorkOffline = true means OFFLINE
				# PrinterStatus codes: 3=Idle, 4=Printing, 5=WarmingUp (good)
				#                    6=Offline, 7=Paused, 8=Error, 10=NotAvailable, 15=PendingDeletion (bad)
				
				# Primary check: WorkOffline flag
				# For USB printers, Windows may cache state, so we need stricter checks
				$portName = $freshPrinter.PortName
				$isUSB = $portName -like "USB*"
				
				if ($freshPrinter.WorkOffline -eq $true) {
					# WorkOffline = true → Printer is OFFLINE/DISCONNECTED
					$isAvailable = $false
					$status = "offline"
				} elseif ($freshPrinter.PrinterStatus -in @(6, 7, 8, 10, 15)) {
					# PrinterStatus indicates error/offline states
					$isAvailable = $false
					if ($freshPrinter.PrinterStatus -eq 6) { $status = "offline" }
					elseif ($freshPrinter.PrinterStatus -eq 7) { $status = "paused" }
					elseif ($freshPrinter.PrinterStatus -eq 8) { $status = "error" }
					elseif ($freshPrinter.PrinterStatus -eq 10) { $status = "not_available" }
					elseif ($freshPrinter.PrinterStatus -eq 15) { $status = "pending_deletion" }
				} elseif ($isUSB) {
					# For USB printers: WorkOffline=false and good status, but verify with fresh query
					# Windows caches USB printer state, so do a fresh query to verify
					try {
						# Force a fresh query (not cached) by querying again
						$freshCheck = Get-WmiObject -Class Win32_Printer -Filter "Name='$($freshPrinter.Name)'" -ErrorAction Stop
						
						# For USB printers, require ALL of these to be true:
						# 1. WorkOffline must be false
						# 2. PrinterStatus must be good (3, 4, 5, etc.)
						# 3. Availability should be 3 (Available) or null
						# 4. Fresh query must succeed
						if ($freshCheck.WorkOffline -eq $false -and 
						    $freshCheck.PrinterStatus -notin @(6, 7, 8, 10, 15) -and
						    ($freshCheck.Availability -eq 3 -or $freshCheck.Availability -eq $null)) {
							$isAvailable = $true
							if ($freshCheck.PrinterStatus -eq 3) { $status = "idle" }
							elseif ($freshCheck.PrinterStatus -eq 4) { $status = "printing" }
							elseif ($freshCheck.PrinterStatus -eq 5) { $status = "warming_up" }
							else { $status = "online" }
						} else {
							# Fresh check failed or indicates offline
							$isAvailable = $false
							$status = "offline"
						}
					} catch {
						# If fresh query fails, printer is likely disconnected
						$isAvailable = $false
						$status = "offline"
					}
					# For network/local printers (non-USB), WorkOffline=false and good status = available
					$isAvailable = $true
					if ($freshPrinter.PrinterStatus -eq 3) { $status = "idle" }
					elseif ($freshPrinter.PrinterStatus -eq 4) { $status = "printing" }
					elseif ($freshPrinter.PrinterStatus -eq 5) { $status = "warming_up" }
					else { $status = "online" }
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

		// Fallback method removed - we cannot reliably detect disconnected printers
		// without WorkOffline flag, so we return empty list to avoid showing disconnected printers
		// If enhanced detection fails, return empty list rather than potentially showing disconnected printers
		log.Printf("WARNING: Enhanced printer detection failed, returning empty list to avoid showing disconnected printers")
	}

	return []map[string]interface{}{}, fmt.Errorf("failed to get printers")
}

