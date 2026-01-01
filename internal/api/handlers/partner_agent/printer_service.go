package partner_agent

import (
	"context"
	"encoding/json"
	"log"
)

// GetSyncedPrinters returns the current synced printer list (thread-safe)
// Falls back to Redis if in-memory list is empty (e.g., after server restart)
func (h *AgentHandler) GetSyncedPrinters() []map[string]interface{} {
	var printers []map[string]interface{}
	
	h.printerMutex.RLock()
	if h.syncedPrinters != nil && len(h.syncedPrinters) > 0 {
		// Create a deep copy to avoid race conditions
		printers = make([]map[string]interface{}, len(h.syncedPrinters))
		for i, p := range h.syncedPrinters {
			// Deep copy each printer map
			printerCopy := make(map[string]interface{})
			for k, v := range p {
				printerCopy[k] = v
			}
			printers[i] = printerCopy
		}
	}
	h.printerMutex.RUnlock()

	log.Printf("DEBUG: GetSyncedPrinters - In-memory count: %d", len(printers))

	// If in-memory list is empty, try to load from Redis
	if len(printers) == 0 && h.redisClient != nil {
		printers = h.loadPrintersFromRedis()
	}

	// Ensure we return a non-nil slice
	if printers == nil {
		printers = []map[string]interface{}{}
		log.Printf("DEBUG: Returning empty printer list (nil was converted to empty slice)")
	}

	log.Printf("DEBUG: GetSyncedPrinters returning %d printers", len(printers))
	return printers
}

// loadPrintersFromRedis loads printers from Redis cache
func (h *AgentHandler) loadPrintersFromRedis() []map[string]interface{} {
	ctx := context.Background()
	printerKey := "partner:printers"
	
	log.Printf("DEBUG: In-memory list is empty, checking Redis for key: %s", printerKey)
	cachedJSON, err := h.redisClient.Get(ctx, printerKey)
	if err == nil && cachedJSON != "" {
		log.Printf("DEBUG: Found printers in Redis (length: %d), attempting to unmarshal...", len(cachedJSON))
		var cachedPrinters []map[string]interface{}
		if err := json.Unmarshal([]byte(cachedJSON), &cachedPrinters); err == nil {
			log.Printf("INFO: Successfully loaded %d printers from Redis cache", len(cachedPrinters))
			if len(cachedPrinters) > 0 {
				h.logPrintersFromRedis(cachedPrinters)
			}
			// Update in-memory cache
			h.printerMutex.Lock()
			h.syncedPrinters = cachedPrinters
			// Create deep copy for return
			printers := make([]map[string]interface{}, len(cachedPrinters))
			for i, p := range cachedPrinters {
				printerCopy := make(map[string]interface{})
				for k, v := range p {
					printerCopy[k] = v
				}
				printers[i] = printerCopy
			}
			h.printerMutex.Unlock()
			log.Printf("DEBUG: Returning %d printers from Redis", len(printers))
			if len(printers) > 0 {
				log.Printf("DEBUG: Sample printer being returned: %+v", printers[0])
			}
			return printers
		} else {
			log.Printf("ERROR: Failed to unmarshal printers from Redis: %v", err)
			if len(cachedJSON) > 0 {
				previewLen := min(200, len(cachedJSON))
				log.Printf("DEBUG: Redis JSON content (first %d chars): %s", previewLen, cachedJSON[:previewLen])
			}
		}
	} else {
		if err != nil {
			log.Printf("WARNING: Redis Get error for key %s: %v", printerKey, err)
		} else {
			log.Printf("INFO: Redis key %s exists but is empty", printerKey)
		}
	}

	return nil
}

// logPrintersFromRedis logs printers loaded from Redis
func (h *AgentHandler) logPrintersFromRedis(cachedPrinters []map[string]interface{}) {
	log.Printf("=========================================")
	log.Printf("PRINTER LIST LOADED FROM REDIS:")
	log.Printf("=========================================")
	for i, printer := range cachedPrinters {
		log.Printf("Printer #%d:", i+1)
		printerJSON, _ := json.MarshalIndent(printer, "  ", "  ")
		log.Printf("  %s", string(printerJSON))
	}
	log.Printf("=========================================")
	log.Printf("Total printers from Redis: %d", len(cachedPrinters))
	log.Printf("=========================================")
}

// min helper function
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

