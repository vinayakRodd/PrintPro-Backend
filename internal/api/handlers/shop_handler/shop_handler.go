package shop_handler

import (
	"encoding/json"
	"log"
	"net/http"
	"print-pro-backend/internal/middleware/auth_middleware"
	"print-pro-backend/internal/repositories"
)

// ShopHandler handles shop-related requests
type ShopHandler struct {
	partnerProfileRepository *repositories.PartnerProfileRepository
}

// NewShopHandler creates a new shop handler
func NewShopHandler(partnerProfileRepository *repositories.PartnerProfileRepository) *ShopHandler {
	return &ShopHandler{
		partnerProfileRepository: partnerProfileRepository,
	}
}

// ShopName represents a shop name response
type ShopName struct {
	ShopName string `json:"shop_name"`
	ID       int64  `json:"id"`
}

// GetShopNamesResponse represents the response for getting shop names
type GetShopNamesResponse struct {
	Success bool       `json:"success"`
	Message string     `json:"message"`
	Shops   []ShopName `json:"shops"`
	Count   int        `json:"count"`
}

// GetShopNames retrieves all shop names from partner_profiles table
// GET /api/shops/names
func (h *ShopHandler) GetShopNames(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.sendErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed", "Only GET method is allowed")
		return
	}

	// Get user from context (optional - can be public or authenticated)
	user, ok := auth_middleware.GetUserFromContext(r)
	if ok {
		log.Printf("INFO: GetShopNames called by user: %s (type: %s)", user.ID, user.UserType)
	} else {
		log.Printf("INFO: GetShopNames called by unauthenticated user")
	}

	// Get all shop names from partner_profiles
	ctx := r.Context()
	shopResults, err := h.partnerProfileRepository.GetAllShopNames(ctx)
	if err != nil {
		log.Printf("ERROR: Failed to get shop names - %v", err)
		h.sendErrorResponse(w, http.StatusInternalServerError, "Failed to retrieve shop names", err.Error())
		return
	}

	// Convert repository results to response format
	shops := make([]ShopName, len(shopResults))
	for i, result := range shopResults {
		shops[i] = ShopName{
			ShopName: result.ShopName,
			ID:       result.ID,
		}
	}

	// Prepare response
	response := GetShopNamesResponse{
		Success: true,
		Message: "Shop names retrieved successfully",
		Shops:   shops,
		Count:   len(shops),
	}

	log.Printf("SUCCESS: Returning %d shop names", len(shops))
	h.sendJSONResponse(w, http.StatusOK, response)
}

// sendJSONResponse sends a JSON response
func (h *ShopHandler) sendJSONResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("ERROR: Failed to encode JSON response - %v", err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// sendErrorResponse sends an error JSON response
func (h *ShopHandler) sendErrorResponse(w http.ResponseWriter, statusCode int, message, error string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": false,
		"message": message,
		"error":   error,
	})
}

