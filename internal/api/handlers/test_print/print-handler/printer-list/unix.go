package printer_list

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// GetUnixPrinters gets list of printers on Linux/Mac
func GetUnixPrinters() ([]map[string]interface{}, error) {
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

