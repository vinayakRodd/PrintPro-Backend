package partner_agent

import (
	"context"
	"encoding/json"
	"log"
)

// GetSyncedPrinters returns the current synced printer list from Redis
// ALWAYS reads from Redis - if Redis is empty, returns empty list
func (h *AgentHandler) GetSyncedPrinters() []map[string]interface{} {
	log.Printf("=========================================")
	log.Printf("GetSyncedPrinters CALLED - Reading from Redis")
	log.Printf("=========================================")
	
	ctx := context.Background()
	printerKey := "partner:printers"
	
	// ALWAYS read from Redis
	if h.redisClient == nil {
		log.Printf("ERROR: Redis client is nil, returning empty list")
		return []map[string]interface{}{}
	}
	
	log.Printf("DEBUG: GetSyncedPrinters - Reading from Redis key: %s", printerKey)
	cachedJSON, err := h.redisClient.Get(ctx, printerKey)
	if err != nil {
		// Key doesn't exist or expired - return empty list
		log.Printf("INFO: Redis key %s does not exist or expired - returning empty list", printerKey)
		return []map[string]interface{}{}
	}
	
	// Check if Redis returned empty string
	if cachedJSON == "" {
		log.Printf("INFO: Redis key %s is empty string - returning empty list", printerKey)
		return []map[string]interface{}{}
	}
	
	log.Printf("DEBUG: GetSyncedPrinters - Found data in Redis (length: %d bytes)", len(cachedJSON))
	log.Printf("DEBUG: GetSyncedPrinters - Redis JSON content: '%s'", cachedJSON)
	
	// Check if it's explicitly an empty array
	if cachedJSON == "[]" || cachedJSON == "null" {
		log.Printf("INFO: GetSyncedPrinters - Redis contains empty array [] - returning empty list")
		log.Printf("=========================================")
		log.Printf("GetSyncedPrinters RETURNING: 0 printers (empty array from Redis)")
		log.Printf("=========================================")
		return []map[string]interface{}{}
	}
	
	var printers []map[string]interface{}
	if err := json.Unmarshal([]byte(cachedJSON), &printers); err != nil {
		log.Printf("ERROR: Failed to unmarshal printers from Redis: %v - returning empty list", err)
		return []map[string]interface{}{}
	}
	
	// Check if unmarshaled array is empty (partner agent sent empty list)
	if len(printers) == 0 {
		log.Printf("INFO: GetSyncedPrinters - Redis contains EMPTY printer list (partner agent sent empty list)")
		log.Printf("=========================================")
		log.Printf("GetSyncedPrinters RETURNING: 0 printers (empty list from Redis)")
		log.Printf("=========================================")
		return []map[string]interface{}{}
	}
	
	log.Printf("SUCCESS: GetSyncedPrinters - Loaded %d printers from Redis", len(printers))
	log.Printf("DEBUG: GetSyncedPrinters - Sample printer from Redis: %+v", printers[0])
	log.Printf("DEBUG: GetSyncedPrinters - All printers from Redis:")
	for i, p := range printers {
		printerJSON, _ := json.MarshalIndent(p, "    ", "  ")
		log.Printf("  Printer #%d:\n%s", i+1, string(printerJSON))
	}
	
	log.Printf("=========================================")
	log.Printf("GetSyncedPrinters RETURNING: %d printers from Redis", len(printers))
	log.Printf("=========================================")
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

