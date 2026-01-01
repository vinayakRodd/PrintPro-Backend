package print_handler

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
)

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

	// PowerShell script to print directly to printer using Windows Print Spooler API
	// This uses .NET System.Drawing.Printing to send file directly to printer without opening applications
	psCommand := fmt.Sprintf(`
		$printerName = "%s"
		$filePath = "%s"
		
		# Verify file exists
		if (-not (Test-Path $filePath)) {
			Write-Error "File not found: $filePath"
			exit 1
		}
		
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
		
		try {
			# Check file extension to determine best printing method
			$fileExt = [System.IO.Path]::GetExtension($filePath).ToLower()
			$isPostScript = $fileExt -eq ".ps"
			
			# For PostScript files, try Windows print command first (more reliable, no dialogs)
			if ($isPostScript) {
				Write-Host "Detected PostScript file, using Windows print command"
				# Use Start-Process with print verb for PostScript (more reliable than cmd print)
				$printProcess = Start-Process -FilePath $filePath -Verb Print -PassThru -WindowStyle Hidden
				if ($printProcess) {
					Write-Host "Print command sent via Start-Process with Print verb"
					# Wait for process and job to appear
					Start-Sleep -Milliseconds 2000
					$printJobs = Get-WmiObject -Class Win32_PrintJob | Where-Object { $_.PrinterName -eq $printerName }
					if ($printJobs) {
						$jobCount = ($printJobs | Measure-Object).Count
						Write-Host "SUCCESS: Print job queued via Start-Process - Found $jobCount job(s)"
						exit 0
					} else {
						Write-Host "INFO: Print command sent but no job in queue (may have printed immediately)"
						exit 0
					}
				} else {
					Write-Host "Start-Process failed, falling back to Shell.Application"
				}
			}
			
			# Method 1: Try using Shell.Application (works for Word, PostScript, and other files)
			$shell = New-Object -ComObject Shell.Application
			$folder = $shell.NameSpace((Split-Path -Parent $filePath))
			$file = $folder.ParseName((Split-Path -Leaf $filePath))
			
			if ($file) {
				# Get initial print job count before printing
				$initialJobs = (Get-WmiObject -Class Win32_PrintJob | Where-Object { $_.PrinterName -eq $printerName } | Measure-Object).Count
				
				# Invoke print verb - Windows will use the default handler and print processor
				$file.InvokeVerb("print")
				Write-Host "Print command sent via Shell.Application"
				
				# Wait for print job to be queued (check multiple times with increasing delays)
				$maxAttempts = 10
				$attempt = 0
				$jobFound = $false
				
				while ($attempt -lt $maxAttempts -and -not $jobFound) {
					Start-Sleep -Milliseconds 500
					$currentJobs = Get-WmiObject -Class Win32_PrintJob | Where-Object { $_.PrinterName -eq $printerName }
					$jobCount = ($currentJobs | Measure-Object).Count
					
					if ($jobCount -gt $initialJobs) {
						$jobFound = $true
						Write-Host "SUCCESS: Print job queued - Found $jobCount job(s) (was $initialJobs before)"
						break
					}
					
					$attempt++
				}
				
				if (-not $jobFound) {
					# Final check after longer wait
					Start-Sleep -Milliseconds 2000
					$finalJobs = Get-WmiObject -Class Win32_PrintJob | Where-Object { $_.PrinterName -eq $printerName }
					$finalJobCount = ($finalJobs | Measure-Object).Count
					
					if ($finalJobCount -gt $initialJobs) {
						Write-Host "SUCCESS: Print job queued - Found $finalJobCount job(s) (was $initialJobs before)"
						$jobFound = $true
					}
				}
				
				if (-not $jobFound) {
					# Check if any print jobs exist at all (might have printed immediately)
					$allJobs = Get-WmiObject -Class Win32_PrintJob | Where-Object { $_.PrinterName -eq $printerName }
					if ($allJobs) {
						Write-Host "INFO: Print job may have completed immediately (found $($allJobs.Count) job(s) in queue)"
					} else {
						Write-Host "WARNING: Print command sent but no print job found in queue after $($maxAttempts * 500 + 2000)ms"
						Write-Host "INFO: This may indicate a dialog opened or the print handler failed"
						# Don't exit with error - the print might still work, just not detectable
					}
				}
			} else {
				Write-Error "Failed to get file object from Shell.Application"
				exit 1
			}
		} catch {
			Write-Error "Failed to print via Shell.Application: $_"
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
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	stdoutStr := stdout.String()
	stderrStr := stderr.String()
	
	// Log output for debugging
	if stdoutStr != "" {
		log.Printf("DEBUG: PowerShell print command output: %s", stdoutStr)
	}
	if stderrStr != "" {
		log.Printf("DEBUG: PowerShell print command error: %s", stderrStr)
	}
	
	if err != nil {
		return fmt.Errorf("print command failed: %v, stdout: %s, stderr: %s", err, stdoutStr, stderrStr)
	}

	return nil
}

// findGhostscript finds the Ghostscript executable on Windows
// Prioritizes PATH lookup first, then falls back to common installation paths
func findGhostscript() (string, error) {
	// First, try to find in PATH using exec.LookPath (most reliable)
	// This will work if Ghostscript bin folder is in System PATH
	gsCommands := []string{"gswin64c", "gswin32c", "gs"}
	for _, cmd := range gsCommands {
		if path, err := exec.LookPath(cmd); err == nil {
			// Verify it's actually Ghostscript by checking version
			if testCmd := exec.Command(path, "--version"); testCmd.Run() == nil {
				log.Printf("Found Ghostscript in PATH: %s", path)
				return path, nil
			}
		}
	}

	// Fallback: Check common installation paths
	gsPaths := []string{
		`C:\Program Files\gs\gs10.04.0\bin\gswin64c.exe`,
		`C:\Program Files\gs\gs10.03.0\bin\gswin64c.exe`,
		`C:\Program Files\gs\gs10.02.1\bin\gswin64c.exe`,
		`C:\Program Files\gs\gs10.02.0\bin\gswin64c.exe`,
		`C:\Program Files\gs\gs10.01.2\bin\gswin64c.exe`,
		`C:\Program Files\gs\gs10.01.1\bin\gswin64c.exe`,
		`C:\Program Files\gs\gs10.01.0\bin\gswin64c.exe`,
		`C:\Program Files\gs\gs10.00.0\bin\gswin64c.exe`,
		`C:\Program Files\gs\gs9.56.1\bin\gswin64c.exe`,
		`C:\Program Files\gs\gs9.56.0\bin\gswin64c.exe`,
		`C:\Program Files\gs\gs9.55.0\bin\gswin64c.exe`,
		`C:\Program Files\gs\gs9.54.0\bin\gswin64c.exe`,
		`C:\Program Files\gs\gs9.53.3\bin\gswin64c.exe`,
		`C:\Program Files\gs\gs9.52\bin\gswin64c.exe`,
		`C:\Program Files\gs\gs9.50\bin\gswin64c.exe`,
		`C:\Program Files (x86)\gs\gs10.04.0\bin\gswin32c.exe`,
		`C:\Program Files (x86)\gs\gs10.03.0\bin\gswin32c.exe`,
		`C:\Program Files (x86)\gs\gs9.56.1\bin\gswin32c.exe`,
		`C:\Program Files (x86)\gs\gs9.56.0\bin\gswin32c.exe`,
		`C:\Program Files (x86)\gs\gs9.55.0\bin\gswin32c.exe`,
		`C:\Program Files (x86)\gs\gs9.54.0\bin\gswin32c.exe`,
		`C:\Program Files (x86)\gs\gs9.53.3\bin\gswin32c.exe`,
		`C:\Program Files (x86)\gs\gs9.52\bin\gswin32c.exe`,
		`C:\Program Files (x86)\gs\gs9.50\bin\gswin32c.exe`,
	}

	for _, path := range gsPaths {
		if _, err := os.Stat(path); err == nil {
			// Test if it's executable
			if testCmd := exec.Command(path, "--version"); testCmd.Run() == nil {
				log.Printf("Found Ghostscript in installation path: %s", path)
				return path, nil
			}
		}
	}

	return "", fmt.Errorf("Ghostscript not found. Please ensure Ghostscript is installed and the bin folder is added to your System PATH. Download from https://www.ghostscript.com/download/gsdnld.html")
}

// printWindowsPDF prints PDF files directly to printer using Ghostscript
// This bypasses the need for PostScript files and Windows file associations
func (h *PrintHandler) printWindowsPDF(filePath, printerName string) error {
	// Verify file exists
	fileInfo, err := os.Stat(filePath)
	if os.IsNotExist(err) {
		return fmt.Errorf("PDF file not found: %s", filePath)
	}
	if err != nil {
		return fmt.Errorf("failed to stat file: %v", err)
	}

	log.Printf("DEBUG: Printing PDF file - Path: %s, Size: %d bytes, Printer: %s", filePath, fileInfo.Size(), printerName)

	// Find Ghostscript
	gsPath, err := findGhostscript()
	if err != nil {
		return fmt.Errorf("Ghostscript not found: %v. Please install Ghostscript from https://www.ghostscript.com/download/gsdnld.html", err)
	}

	// Use Ghostscript to print directly to the printer (bypasses PostScript file and Windows file associations)
	// -dNOPAUSE: Don't pause between pages (bypasses UI prompts)
	// -dBATCH: Exit after processing (prevents interactive mode)
	// -dSAFER: Security flag (restricts file operations)
	// -sDEVICE=mswinpr2: Use Windows printer device (direct printing, no dialogs)
	// -sOutputFile="%%printer%%PrinterName": Send output directly to printer (bypasses "Save As" prompt)
	// The %%printer%% prefix tells Ghostscript to print directly to the named printer
	printerOutput := fmt.Sprintf("%%printer%%%s", printerName)
	
	log.Printf("DEBUG: Ghostscript command - Executable: %s, Device: mswinpr2, Printer: %s, Input file: %s", gsPath, printerName, filePath)
	
	gsCmd := exec.Command(gsPath,
		"-dNOPAUSE",
		"-dBATCH",
		"-dSAFER",
		"-sDEVICE=mswinpr2",
		"-sOutputFile="+printerOutput,
		filePath,
	)

	var gsStdout, gsStderr bytes.Buffer
	gsCmd.Stdout = &gsStdout
	gsCmd.Stderr = &gsStderr

	if err := gsCmd.Run(); err != nil {
		gsError := gsStderr.String()
		gsOutput := gsStdout.String()
		log.Printf("DEBUG: Ghostscript print error: %s", gsError)
		log.Printf("DEBUG: Ghostscript print output: %s", gsOutput)
		return fmt.Errorf("failed to print PDF via Ghostscript: %v, stderr: %s, stdout: %s", err, gsError, gsOutput)
	}

	log.Printf("PDF sent to printer successfully via Ghostscript direct printing")
	return nil
}

