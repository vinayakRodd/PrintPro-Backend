package partner_agent

import (
	"context"
	"encoding/json"
	"log"
	"time"
)

// storePrinters stores printers EXACTLY as received from partner agent
// NO local detection, NO transformation - stores exactly what partner agent sends
// These printers will be returned to frontend via GetSyncedPrinters() -> ListPrinters()
func (h *AgentHandler) storePrinters(printersList []map[string]interface{}) {
	log.Printf("INFO: storePrinters - Storing %d printers EXACTLY as received from partner agent (NO local detection)", len(printersList))

	// Store the synced printer list in memory (thread-safe)
	// Create a deep copy to avoid reference issues, but preserve ALL fields exactly
	log.Printf("=========================================")
	log.Printf("storePrinters - Starting storage of %d printers", len(printersList))
	log.Printf("=========================================")
	log.Printf("DEBUG: storePrinters - h.syncedPrinters pointer before lock: %p", h.syncedPrinters)

	h.printerMutex.Lock()
	log.Printf("DEBUG: storePrinters - Acquired write lock")
	h.syncedPrinters = make([]map[string]interface{}, len(printersList))
	log.Printf("DEBUG: storePrinters - Created new slice, h.syncedPrinters pointer: %p", h.syncedPrinters)

	for i, p := range printersList {
		// Deep copy - preserve ALL fields exactly as sent by partner agent
		printerCopy := make(map[string]interface{})
		for k, v := range p {
			printerCopy[k] = v
		}
		h.syncedPrinters[i] = printerCopy
		log.Printf("DEBUG: storePrinters - Stored printer #%d: %+v", i+1, printerCopy)
	}
	printerCount := len(h.syncedPrinters)
	log.Printf("INFO: storePrinters - Stored %d printers in memory", printerCount)
	log.Printf("DEBUG: storePrinters - h.syncedPrinters length after storage: %d", len(h.syncedPrinters))

	if printerCount > 0 {
		log.Printf("DEBUG: storePrinters - Sample stored printer: %+v", h.syncedPrinters[0])
		log.Printf("DEBUG: storePrinters - All stored printers:")
		for i, p := range h.syncedPrinters {
			printerJSON, _ := json.MarshalIndent(p, "    ", "  ")
			log.Printf("  Printer #%d:\n%s", i+1, string(printerJSON))
		}
	}
	h.printerMutex.Unlock()
	log.Printf("DEBUG: storePrinters - Released write lock")

	log.Printf("SUCCESS: Printers stored in memory - Count: %d (will be returned EXACTLY as stored to frontend)", printerCount)

	// Verify printers are actually stored (after unlock)
	h.printerMutex.RLock()
	verifyCount := len(h.syncedPrinters)
	if verifyCount > 0 {
		log.Printf("DEBUG: Verification - Sample printer from memory: %+v", h.syncedPrinters[0])
		log.Printf("DEBUG: Verification - All printers in memory:")
		for i, p := range h.syncedPrinters {
			log.Printf("  Printer #%d: %+v", i+1, p)
		}
	} else {
		log.Printf("WARNING: Verification failed - Printers count is 0 after storing!")
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

