package printer_handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"print-pro-backend/internal/infrastructure"
	"print-pro-backend/internal/middleware/auth_middleware"
	"print-pro-backend/internal/models/printer"
	"print-pro-backend/internal/repositories"
	"time"
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

// PrinterHandler handles printer-related requests
type PrinterHandler struct {
	printerRepository         *repositories.PrinterRepository
	partnerProfileRepository   *repositories.PartnerProfileRepository
	redisClient               *infrastructure.RedisClient
}

// NewPrinterHandler creates a new printer handler
func NewPrinterHandler(
	printerRepository *repositories.PrinterRepository,
	partnerProfileRepository *repositories.PartnerProfileRepository,
	redisClient *infrastructure.RedisClient,
) *PrinterHandler {
	return &PrinterHandler{
		printerRepository:       printerRepository,
		partnerProfileRepository: partnerProfileRepository,
		redisClient:             redisClient,
	}
}

// UpdatePrintersHandler receives printer updates from the agent
func (h *PrinterHandler) UpdatePrintersHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	
	fmt.Println("🔔 Ping received from Agent!")
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload AgentPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		fmt.Printf("❌ ERROR: Failed to decode printer payload - %v\n", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Print detailed information about received printers
	fmt.Printf("✅ Received printer update from Partner ID: %s\n", payload.PartnerID)
	fmt.Printf("📊 Total printers detected: %d\n", len(payload.Printers))

	// Get partner profile by printer_id (the unique agent identifier)
	partnerProfile, err := h.partnerProfileRepository.GetByPrinterID(ctx, payload.PartnerID)
	if err != nil {
		fmt.Printf("❌ ERROR: Partner profile not found for printer_id: %s - %v\n", payload.PartnerID, err)
		http.Error(w, fmt.Sprintf("Partner not found: %s", payload.PartnerID), http.StatusNotFound)
		return
	}

	fmt.Printf("✅ Found partner profile: ID=%d, Shop=%s\n", partnerProfile.ID, partnerProfile.ShopName)

	// Process each printer
	successCount := 0
	errorCount := 0

	for i, p := range payload.Printers {
		// Use port as serial_number (or name+port combination for uniqueness)
		serialNumber := fmt.Sprintf("%s-%s", p.Name, p.Port)
		
		// Normalize status
		status := p.Status
		if status == "" {
			status = "online"
		}

		// Upsert printer to database
		printer, err := h.printerRepository.UpsertPrinter(ctx, partnerProfile.ID, p.Name, serialNumber, status)
		if err != nil {
			fmt.Printf("❌ ERROR: Failed to upsert printer [%d] %s - %v\n", i+1, p.Name, err)
			errorCount++
			continue
		}

		fmt.Printf("   [%d] ✅ Saved: %s (Serial: %s, Status: %s, ID: %d)\n", 
			i+1, printer.PrinterName, printer.SerialNumber, printer.Status, printer.ID)

		// Update Redis heartbeat for real-time status tracking
		// Key format: printer:heartbeat:{partner_id}:{serial_number}
		heartbeatKey := fmt.Sprintf("printer:heartbeat:%d:%s", partnerProfile.ID, serialNumber)
		heartbeatValue := time.Now().Format(time.RFC3339)
		
		// Set heartbeat with 5 minute expiration (printer considered offline if no update in 5 min)
		if err := h.redisClient.Set(ctx, heartbeatKey, heartbeatValue, 5*time.Minute); err != nil {
			fmt.Printf("⚠️  WARNING: Failed to update Redis heartbeat for printer %s - %v\n", serialNumber, err)
		}

		successCount++
	}

	// Summary
	if errorCount > 0 {
		fmt.Printf("⚠️  WARNING: %d printers failed to save, %d succeeded\n", errorCount, successCount)
	} else {
		fmt.Printf("✅ All %d printers synced successfully!\n", successCount)
	}

	fmt.Printf("✅ Sync Successful - Response sent to Partner: %s\n", payload.PartnerID)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Sync Successful"))
}

// GetPrintersResponse represents the response for getting printers
type GetPrintersResponse struct {
	Success bool                      `json:"success"`
	Message string                    `json:"message"`
	Printers []printer.Printer        `json:"printers"`
	Count   int                       `json:"count"`
}

// GetPrintersHandler retrieves all printers for the authenticated partner
func (h *PrinterHandler) GetPrintersHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Only allow GET requests
	if r.Method != http.MethodGet {
		h.sendErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed", "Only GET method is allowed")
		return
	}

	// Get user from context (set by auth middleware)
	user, ok := auth_middleware.GetUserFromContext(r)
	if !ok {
		h.sendErrorResponse(w, http.StatusUnauthorized, "Unauthorized", "User not found in session")
		return
	}

	// Verify user is a partner
	if user.UserType != "partner" {
		h.sendErrorResponse(w, http.StatusForbidden, "Forbidden", "Only partners can access printer data")
		return
	}

	// Convert user.ID (string) to int (account_id)
	accountID, err := strconv.Atoi(user.ID)
	if err != nil {
		fmt.Printf("❌ ERROR: Invalid account ID format: %s - %v\n", user.ID, err)
		h.sendErrorResponse(w, http.StatusBadRequest, "Invalid user ID", "Invalid account ID format")
		return
	}

	// Get partner profile by account_id
	partnerProfile, err := h.partnerProfileRepository.GetByAccountID(ctx, accountID)
	if err != nil {
		fmt.Printf("❌ ERROR: Partner profile not found for account_id: %d - %v\n", accountID, err)
		h.sendErrorResponse(w, http.StatusNotFound, "Partner profile not found", "Partner profile not found for this account")
		return
	}

	// Get all printers for this partner
	printers, err := h.printerRepository.GetByPartnerID(ctx, partnerProfile.ID)
	if err != nil {
		fmt.Printf("❌ ERROR: Failed to get printers for partner_id: %d - %v\n", partnerProfile.ID, err)
		h.sendErrorResponse(w, http.StatusInternalServerError, "Failed to retrieve printers", err.Error())
		return
	}

	// Prepare response
	response := GetPrintersResponse{
		Success:  true,
		Message:  "Printers retrieved successfully",
		Printers: printers,
		Count:    len(printers),
	}

	// Send JSON response
	h.sendJSONResponse(w, http.StatusOK, response)
}

// sendJSONResponse sends a JSON response
func (h *PrinterHandler) sendJSONResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		fmt.Printf("❌ ERROR: Failed to encode JSON response - %v\n", err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// sendErrorResponse sends an error JSON response
func (h *PrinterHandler) sendErrorResponse(w http.ResponseWriter, statusCode int, message, error string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": false,
		"message": message,
		"error":   error,
	})
}


