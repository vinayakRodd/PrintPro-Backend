package partner_agent

import (
	"encoding/json"
	"log"
)

// logPrinterList logs the entire printer list for debugging
func (h *AgentHandler) logPrinterList(printersList []map[string]interface{}) {
	log.Printf("=========================================")
	log.Printf("LOG PRINTER LIST (Detailed)")
	log.Printf("=========================================")
	if len(printersList) > 0 {
		log.Printf("PRINTER LIST RECEIVED FROM PARTNER AGENT:")
		for i, printer := range printersList {
			log.Printf("Printer #%d:", i+1)
			printerJSON, _ := json.MarshalIndent(printer, "  ", "  ")
			log.Printf("  %s", string(printerJSON))
		}
		log.Printf("Total printers: %d", len(printersList))
	} else {
		log.Printf("EMPTY PRINTER LIST received from Partner Agent")
		log.Printf("WARNING: Empty printer list received from partner agent")
	}
	log.Printf("=========================================")
}

