package partner_agent

import (
	"log"
)

// convertPrintersToStandardFormat converts various printer formats to standardized format
func (h *AgentHandler) convertPrintersToStandardFormat(printers interface{}) []map[string]interface{} {
	var printersList []map[string]interface{}

	log.Printf("DEBUG: convertPrintersToStandardFormat - Input type: %T", printers)
	log.Printf("DEBUG: convertPrintersToStandardFormat - Input value: %+v", printers)

	if printers == nil {
		log.Printf("WARNING: Printers list is nil, initializing empty array")
		return []map[string]interface{}{}
	}

	// Check if it's an array of strings or array of objects
	switch v := printers.(type) {
	case []interface{}:
		// Handle both []string and []map[string]interface{}
		// If empty array, return empty list (not nil)
		if len(v) == 0 {
			log.Printf("DEBUG: convertPrintersToStandardFormat - Empty []interface{} received, returning empty array")
			return []map[string]interface{}{}
		}

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
		// If empty array, return empty list (not nil)
		if len(v) == 0 {
			log.Printf("DEBUG: convertPrintersToStandardFormat - Empty []string received, returning empty array")
			return []map[string]interface{}{}
		}

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

	log.Printf("DEBUG: convertPrintersToStandardFormat - Returning %d printers", len(printersList))
	return printersList
}

