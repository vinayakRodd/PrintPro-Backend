package print_handler

import (
	"bytes"
	"fmt"
	"os/exec"
)

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

