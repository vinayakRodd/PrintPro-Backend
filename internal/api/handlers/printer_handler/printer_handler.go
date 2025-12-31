package printer_handler

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// Printer represents a single printer reported by the agent
type Printer struct {
	Name   string `json:"name"`
	Port   string `json:"port"`
	Status string `json:"status"`
}

// AgentPayload is the payload sent by the agent containing printers for a partner
type AgentPayload struct {
	PartnerID string    `json:"partner_id"`
	Printers  []Printer `json:"printers"`
}

// UpdatePrintersHandler receives printer updates from the agent
func UpdatePrintersHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Ping received from Agent!")
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload AgentPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// For now, just print to the server logs
	fmt.Printf("Received update from Partner: %s\n", payload.PartnerID)
	for _, p := range payload.Printers {
		fmt.Printf(" - Found Printer: %s on %s (status: %s)\n", p.Name, p.Port, p.Status)
	}

	// TODO: Save to Redis/DB when the storage model is defined
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Sync Successful"))
}


