package partner_agent

import (
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

	// Store printers in Redis - ALWAYS update Redis when partner agent sends new list
	ctx := context.Background()
	printerKey := "partner:printers"
	
	// ALWAYS store in Redis - even if empty list
	// This ensures Redis reflects the exact state sent by partner agent
	if len(printersList) == 0 {
		// Partner agent sent empty list - explicitly store empty array and delete any old data
		log.Printf("INFO: Partner agent sent EMPTY list - storing empty array [] in Redis")
		
		if h.redisClient != nil {
			// First, delete the key to ensure no old data remains
			if err := h.redisClient.Delete(ctx, printerKey); err != nil {
				log.Printf("WARNING: Failed to delete old Redis key %s: %v (continuing anyway)", printerKey, err)
			}
			
			// Store empty array [] explicitly
			emptyArrayJSON := "[]"
			if err := h.redisClient.Set(ctx, printerKey, emptyArrayJSON, 5*time.Second); err != nil {
				log.Printf("ERROR: Failed to store EMPTY printer list in Redis: %v", err)
			} else {
				log.Printf("SUCCESS: EMPTY printer list stored in Redis with key: %s (JSON: %s)", printerKey, emptyArrayJSON)
				// Verify it was stored correctly
				verifyJSON, _ := h.redisClient.Get(ctx, printerKey)
				log.Printf("DEBUG: Verification - Redis now contains: %s", verifyJSON)
			}
		} else {
			log.Printf("ERROR: Redis client is nil, cannot store empty list")
		}
	} else {
		// Partner agent sent non-empty list - store normally
		printerJSON, err := json.Marshal(printersList)
		if err != nil {
			log.Printf("ERROR: Failed to marshal printers to JSON: %v", err)
		} else {
			// Store in Redis with 5 second expiration (will be refreshed on next sync)
			if h.redisClient != nil {
				if err := h.redisClient.Set(ctx, printerKey, printerJSON, 5*time.Second); err != nil {
					log.Printf("ERROR: Failed to store printers in Redis: %v", err)
				} else {
					log.Printf("SUCCESS: Printer list stored in Redis with key: %s (Count: %d)", printerKey, len(printersList))
					log.Printf("DEBUG: Redis JSON length: %d bytes", len(printerJSON))
				}
			} else {
				log.Printf("ERROR: Redis client is nil, cannot store printers")
			}
		}
	}
}

