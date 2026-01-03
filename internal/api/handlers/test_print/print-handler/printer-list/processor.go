package printer_list

import (
	"strings"
)

// ProcessPrinterList processes printer list and filters virtual printers
func ProcessPrinterList(printerList []BasicPrinterInfo) ([]map[string]interface{}, error) {
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

		// Check printer status - filter out disconnected/offline printers
		status := "online"
		isAvailable := true
		if p.PrinterStatus != nil {
			statusCode := *p.PrinterStatus
			// Status codes: 6=Offline, 7=Paused, 8=Error, 10=NotAvailable, 15=PendingDeletion
			// Only include printers with good status (3=Idle, 4=Printing, 5=WarmingUp, etc.)
			if statusCode == 6 || statusCode == 7 || statusCode == 8 || statusCode == 10 || statusCode == 15 {
				status = "offline"
				isAvailable = false
			}
		}

		// Skip disconnected/offline printers
		if !isAvailable {
			continue
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

// ProcessPrinterListEnhanced processes enhanced printer list with availability checking
func ProcessPrinterListEnhanced(printerList []PrinterInfo) ([]map[string]interface{}, error) {
	printers := make([]map[string]interface{}, 0)
	for _, p := range printerList {
		// Only include available printers (filter out disconnected ones)
		if !p.IsAvailable {
			// Skip offline/disconnected printers (WorkOffline=true, or status codes 6,7,8,10,15)
			continue
		}

		isDefault := false
		if p.Default != nil && *p.Default {
			isDefault = true
		}

		printerInfo := map[string]interface{}{
			"name":       p.Name,
			"status":     p.Status,
			"is_default": isDefault,
		}

		if p.DriverName != "" {
			printerInfo["driver"] = p.DriverName
		}
		if p.PortName != "" {
			printerInfo["port"] = p.PortName
		}
		if p.Local != nil {
			printerInfo["local"] = *p.Local
		}
		if p.Network != nil {
			printerInfo["network"] = *p.Network
		}

		printers = append(printers, printerInfo)
	}

	return printers, nil
}

