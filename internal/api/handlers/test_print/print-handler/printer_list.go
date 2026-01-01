package print_handler

import (
	printer_list "print-pro-backend/internal/api/handlers/test_print/print-handler/printer-list"
)

// getWindowsPrinters gets list of printers on Windows
func (h *PrintHandler) getWindowsPrinters() ([]map[string]interface{}, error) {
	return printer_list.GetWindowsPrinters()
}

// getUnixPrinters gets list of printers on Linux/Mac
func (h *PrintHandler) getUnixPrinters() ([]map[string]interface{}, error) {
	return printer_list.GetUnixPrinters()
}
