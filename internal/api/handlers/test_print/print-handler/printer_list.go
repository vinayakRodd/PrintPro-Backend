package print_handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

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
					Name          string  `json:"Name"`
					PrinterStatus *int    `json:"PrinterStatus"`
					DriverName    string  `json:"DriverName"`
					PortName      string  `json:"PortName"`
					Default       *bool   `json:"Default"`
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
	Name          string  `json:"Name"`
	PrinterStatus *int    `json:"PrinterStatus"`
	DriverName    string  `json:"DriverName"`
	PortName      string  `json:"PortName"`
	Default       *bool   `json:"Default"`
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

