package partner_agent

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
)

// ConvertPDFToPostScript converts a PDF file to PostScript format using Ghostscript
func ConvertPDFToPostScript(pdfPath, outputDir string) (string, error) {
	// Verify PDF file exists
	fileInfo, err := os.Stat(pdfPath)
	if os.IsNotExist(err) {
		return "", fmt.Errorf("PDF file not found: %s", pdfPath)
	}
	if err != nil {
		return "", fmt.Errorf("failed to stat file: %v", err)
	}

	log.Printf("DEBUG: Converting PDF to PostScript - Path: %s, Size: %d bytes", pdfPath, fileInfo.Size())

	// Find Ghostscript
	gsPath, err := findGhostscript()
	if err != nil {
		return "", fmt.Errorf("Ghostscript not found: %v. Please install Ghostscript from https://www.ghostscript.com/download/gsdnld.html", err)
	}

	// Generate output filename (same name but with .ps extension)
	pdfBaseName := filepath.Base(pdfPath)
	psFileName := filepath.Base(pdfBaseName[:len(pdfBaseName)-len(filepath.Ext(pdfBaseName))]) + ".ps"
	psFilePath := filepath.Join(outputDir, psFileName)

	// Ensure output directory exists
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create output directory: %v", err)
	}

	// Use Ghostscript to convert PDF to PostScript
	// -dNOPAUSE: Don't pause between pages
	// -dBATCH: Exit after processing
	// -dSAFER: Security flag
	// -sDEVICE=ps2write: Use PostScript Level 2 device
	// -sOutputFile: Output file path
	gsCmd := exec.Command(gsPath,
		"-dNOPAUSE",
		"-dBATCH",
		"-dSAFER",
		"-sDEVICE=ps2write",
		"-sOutputFile="+psFilePath,
		pdfPath,
	)

	var gsStdout, gsStderr bytes.Buffer
	gsCmd.Stdout = &gsStdout
	gsCmd.Stderr = &gsStderr

	if err := gsCmd.Run(); err != nil {
		gsError := gsStderr.String()
		gsOutput := gsStdout.String()
		log.Printf("DEBUG: Ghostscript conversion error: %s", gsError)
		log.Printf("DEBUG: Ghostscript conversion output: %s", gsOutput)
		return "", fmt.Errorf("failed to convert PDF to PostScript: %v, stderr: %s, stdout: %s", err, gsError, gsOutput)
	}

	log.Printf("SUCCESS: PDF converted to PostScript: %s", psFilePath)
	return psFileName, nil
}

// findGhostscript finds the Ghostscript executable on Windows
func findGhostscript() (string, error) {
	// First, try to find in PATH
	gsCommands := []string{"gswin64c", "gswin32c", "gs"}
	for _, cmd := range gsCommands {
		if path, err := exec.LookPath(cmd); err == nil {
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
			if testCmd := exec.Command(path, "--version"); testCmd.Run() == nil {
				log.Printf("Found Ghostscript in installation path: %s", path)
				return path, nil
			}
		}
	}

	return "", fmt.Errorf("Ghostscript not found. Please ensure Ghostscript is installed and the bin folder is added to your System PATH. Download from https://www.ghostscript.com/download/gsdnld.html")
}


