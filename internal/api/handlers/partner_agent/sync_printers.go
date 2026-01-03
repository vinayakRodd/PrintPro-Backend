package partner_agent

import (
	"encoding/json"
	"log"
	"net/http"
)

// SyncPrinters handles requests from partner agent to sync printer list
func (h *AgentHandler) SyncPrinters(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req SyncPrintersRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("ERROR: Failed to decode sync printers request: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Log the RAW request from partner agent
	log.Printf("=========================================")
	log.Printf("PARTNER AGENT SYNC REQUEST RECEIVED")
	log.Printf("=========================================")
	reqJSON, _ := json.MarshalIndent(req, "", "  ")
	log.Printf("RAW Request from Partner Agent:\n%s", string(reqJSON))
	log.Printf("Request Printers Type: %T", req.Printers)
	log.Printf("Request Printers Value: %+v", req.Printers)
	log.Printf("=========================================")

	// Convert printers to standardized format (array of maps)
	printersList := h.convertPrintersToStandardFormat(req.Printers)
	if printersList == nil {
		log.Printf("ERROR: SyncPrinters - Invalid printers format, printersList is nil")
		http.Error(w, "Invalid printers format", http.StatusBadRequest)
		return
	}

	log.Printf("=========================================")
	log.Printf("PRINTER LIST FROM PARTNER AGENT")
	log.Printf("=========================================")
	log.Printf("Printer Count: %d", len(printersList))

	if len(printersList) == 0 {
		log.Printf("WARNING: Partner agent sent EMPTY printer list")
	} else {
		log.Printf("Printers received from Partner Agent:")
		for i, printer := range printersList {
			printerJSON, _ := json.MarshalIndent(printer, "    ", "  ")
			log.Printf("  Printer #%d:\n%s", i+1, string(printerJSON))
		}
	}
	log.Printf("=========================================")

	// Print the entire printer list received from partner agent
	h.logPrinterList(printersList)

	// ALWAYS store what partner agent sends - even if empty list
	// This ensures Redis is updated with the latest state (empty or not)
	h.storePrinters(printersList)

	if len(printersList) == 0 {
		log.Printf("INFO: Partner agent sent EMPTY printer list - storing empty list in Redis")
	} else {
		log.Printf("INFO: Printer list synced from partner agent: %d printers", len(printersList))
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":   "success",
		"message":  "Printers synced successfully",
		"count":    len(printersList),
	})
}

