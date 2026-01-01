package partner_agent

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"
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

	// Convert printers to standardized format (array of maps)
	printersList := h.convertPrintersToStandardFormat(req.Printers)
	if printersList == nil {
		http.Error(w, "Invalid printers format", http.StatusBadRequest)
		return
	}

	log.Printf("DEBUG: SyncPrinters received - Printer count: %d", len(printersList))
	
	// Print the entire printer list received from partner agent
	h.logPrinterList(printersList)

	// Store printers in memory and Redis
	h.storePrinters(printersList)

	log.Printf("INFO: Printer list synced from partner agent: %d printers", len(printersList))

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":   "success",
		"message":  "Printers synced successfully",
		"count":    len(printersList),
	})
}

// convertPrintersToStandardFormat converts various printer formats to standardized format
func (h *AgentHandler) convertPrintersToStandardFormat(printers interface{}) []map[string]interface{} {
	var printersList []map[string]interface{}
	
	if printers == nil {
		log.Printf("WARNING: Printers list is nil, initializing empty array")
		return []map[string]interface{}{}
	}

	// Check if it's an array of strings or array of objects
	switch v := printers.(type) {
	case []interface{}:
		// Handle both []string and []map[string]interface{}
		for i, item := range v {
			switch printerItem := item.(type) {
			case string:
				// Simple string format: ["Printer1", "Printer2"]
				printersList = append(printersList, map[string]interface{}{
					"name":   printerItem,
					"status": "unknown",
				})
			case map[string]interface{}:
				// Object format: [{"name": "Printer1", "status": "online"}]
				printersList = append(printersList, printerItem)
			default:
				log.Printf("WARNING: Unknown printer format at index %d: %T", i, printerItem)
			}
		}
	case []string:
		// Direct string array
		for _, name := range v {
			printersList = append(printersList, map[string]interface{}{
				"name":   name,
				"status": "unknown",
			})
		}
	default:
		log.Printf("ERROR: Unexpected printers format: %T", printers)
		return nil
	}

	return printersList
}

// logPrinterList logs the entire printer list for debugging
func (h *AgentHandler) logPrinterList(printersList []map[string]interface{}) {
	if len(printersList) > 0 {
		log.Printf("=========================================")
		log.Printf("PRINTER LIST RECEIVED FROM PARTNER AGENT:")
		log.Printf("=========================================")
		for i, printer := range printersList {
			log.Printf("Printer #%d:", i+1)
			printerJSON, _ := json.MarshalIndent(printer, "  ", "  ")
			log.Printf("  %s", string(printerJSON))
		}
		log.Printf("=========================================")
		log.Printf("Total printers: %d", len(printersList))
		log.Printf("=========================================")
	} else {
		log.Printf("WARNING: Empty printer list received from partner agent")
	}
}

// storePrinters stores printers in memory and Redis
func (h *AgentHandler) storePrinters(printersList []map[string]interface{}) {
	// Store the synced printer list in memory (thread-safe)
	// Create a deep copy to avoid reference issues
	h.printerMutex.Lock()
	h.syncedPrinters = make([]map[string]interface{}, len(printersList))
	for i, p := range printersList {
		printerCopy := make(map[string]interface{})
		for k, v := range p {
			printerCopy[k] = v
		}
		h.syncedPrinters[i] = printerCopy
	}
	printerCount := len(h.syncedPrinters)
	h.printerMutex.Unlock()
	
	log.Printf("DEBUG: Printers stored in memory - Count: %d", printerCount)
	
	// Verify printers are actually stored
	h.printerMutex.RLock()
	verifyCount := len(h.syncedPrinters)
	if verifyCount > 0 {
		log.Printf("DEBUG: Sample printer from memory: %+v", h.syncedPrinters[0])
	}
	h.printerMutex.RUnlock()
	log.Printf("DEBUG: Verification - Printers in memory after store: %d", verifyCount)

	// Store the printer list in Redis with appropriate key
	ctx := context.Background()
	printerKey := "partner:printers"
	
	// Convert printers to JSON for storage
	printerJSON, err := json.Marshal(printersList)
	if err != nil {
		log.Printf("ERROR: Failed to marshal printers to JSON: %v", err)
	} else {
		// Store in Redis with 24 hour expiration (will be refreshed on next sync)
		if h.redisClient != nil {
			if err := h.redisClient.Set(ctx, printerKey, printerJSON, 24*time.Hour); err != nil {
				log.Printf("ERROR: Failed to store printers in Redis: %v", err)
			} else {
				log.Printf("INFO: Printer list stored in Redis with key: %s", printerKey)
				log.Printf("DEBUG: Redis JSON length: %d bytes", len(printerJSON))
				// Print formatted JSON for debugging
				var prettyJSON bytes.Buffer
				json.Indent(&prettyJSON, printerJSON, "", "  ")
				log.Printf("DEBUG: Redis stored JSON (formatted):\n%s", prettyJSON.String())
			}
		} else {
			log.Printf("WARNING: Redis client is nil, cannot store printers")
		}
	}
}

