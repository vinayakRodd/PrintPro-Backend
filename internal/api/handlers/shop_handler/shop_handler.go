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
	shopPreferenceRepository *repositories.ShopPreferenceRepository
	customerProfileRepository *repositories.CustomerProfileRepository
}

// NewShopHandler creates a new shop handler
func NewShopHandler(
	partnerProfileRepository *repositories.PartnerProfileRepository,
	shopPreferenceRepository *repositories.ShopPreferenceRepository,
	customerProfileRepository *repositories.CustomerProfileRepository,
) *ShopHandler {
	return &ShopHandler{
		partnerProfileRepository:  partnerProfileRepository,
		shopPreferenceRepository: shopPreferenceRepository,
		customerProfileRepository: customerProfileRepository,
	}
}

// ShopName represents a shop name response
type ShopName struct {
	ShopName     string `json:"shop_name"`
	AccountEmail string `json:"account_email,omitempty"`
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
		log.Printf("INFO: GetShopNames called by user (type: %s)", user.UserType)
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
			ShopName:     result.ShopName,
			AccountEmail: result.PartnerEmail,
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

// GetShopPreference retrieves the user's selected shop preference
// GET /api/shops/preference
func (h *ShopHandler) GetShopPreference(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.sendErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed", "Only GET method is allowed")
		return
	}

	// Get authenticated user from context
	user, ok := auth_middleware.GetUserFromContext(r)
	if !ok {
		log.Printf("ERROR: GetShopPreference called without authentication")
		h.sendErrorResponse(w, http.StatusUnauthorized, "Unauthorized", "Authentication required")
		return
	}

	// Only customers can have shop preferences
	if user.UserType != "customer" {
		log.Printf("ERROR: GetShopPreference called by non-customer user (type: %s)", user.UserType)
		h.sendErrorResponse(w, http.StatusForbidden, "Forbidden", "Only customers can have shop preferences")
		return
	}

	ctx := r.Context()

	// Get account_email from user.ID (user.ID is the email)
	accountEmail := user.ID

	// Get shop preference by account_email
	preference, err := h.shopPreferenceRepository.GetByAccountEmail(ctx, accountEmail)
	if err != nil {
		// No preference found - return success with null shop
		log.Printf("INFO: No shop preference found for customer")
		h.sendJSONResponse(w, http.StatusOK, map[string]interface{}{
			"success": true,
			"shop":    nil,
		})
		return
	}

	log.Printf("SUCCESS: Shop preference retrieved - shop: %s", preference.ShopName)
	h.sendJSONResponse(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"shop": map[string]interface{}{
			"shop_name": preference.ShopName,
		},
	})
}

// SetShopPreference saves the user's selected shop preference
// POST /api/shops/preference
func (h *ShopHandler) SetShopPreference(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.sendErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed", "Only POST method is allowed")
		return
	}

	// Get authenticated user from context
	user, ok := auth_middleware.GetUserFromContext(r)
	if !ok {
		log.Printf("ERROR: SetShopPreference called without authentication")
		h.sendErrorResponse(w, http.StatusUnauthorized, "Unauthorized", "Authentication required")
		return
	}

	// Only customers can have shop preferences
	if user.UserType != "customer" {
		log.Printf("ERROR: SetShopPreference called by non-customer user (type: %s)", user.UserType)
		h.sendErrorResponse(w, http.StatusForbidden, "Forbidden", "Only customers can have shop preferences")
		return
	}

	// Parse request body
	var req struct {
		ShopName string `json:"shop_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("ERROR: Failed to decode request body: %v", err)
		h.sendErrorResponse(w, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

	if req.ShopName == "" {
		log.Printf("ERROR: Empty shop_name")
		h.sendErrorResponse(w, http.StatusBadRequest, "Invalid shop_name", "shop_name is required")
		return
	}

	ctx := r.Context()

	// Get account_email from user.ID (user.ID is the email)
	accountEmail := user.ID

	// Verify shop exists
	_, err := h.partnerProfileRepository.GetByShopName(ctx, req.ShopName)
	if err != nil {
		log.Printf("ERROR: Shop not found: %s - %v", req.ShopName, err)
		h.sendErrorResponse(w, http.StatusNotFound, "Shop not found", "The specified shop does not exist")
		return
	}

	// Save shop preference (using account_email as PK)
	if err := h.shopPreferenceRepository.Upsert(ctx, accountEmail); err != nil {
		log.Printf("ERROR: Failed to save shop preference: %v", err)
		h.sendErrorResponse(w, http.StatusInternalServerError, "Failed to save shop preference", err.Error())
		return
	}

	log.Printf("SUCCESS: Shop preference saved - shop: %s", req.ShopName)
	h.sendJSONResponse(w, http.StatusOK, map[string]interface{}{
		"success": true,
	})
}

// DeleteShopPreference clears the user's selected shop preference
// DELETE /api/shops/preference
func (h *ShopHandler) DeleteShopPreference(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		h.sendErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed", "Only DELETE method is allowed")
		return
	}

	// Get authenticated user from context
	user, ok := auth_middleware.GetUserFromContext(r)
	if !ok {
		log.Printf("ERROR: DeleteShopPreference called without authentication")
		h.sendErrorResponse(w, http.StatusUnauthorized, "Unauthorized", "Authentication required")
		return
	}

	// Only customers can have shop preferences
	if user.UserType != "customer" {
		log.Printf("ERROR: DeleteShopPreference called by non-customer user (type: %s)", user.UserType)
		h.sendErrorResponse(w, http.StatusForbidden, "Forbidden", "Only customers can have shop preferences")
		return
	}

	ctx := r.Context()

	// Get account_email from user.ID (user.ID is the email)
	accountEmail := user.ID

	// Delete shop preference
	if err := h.shopPreferenceRepository.DeleteByAccountEmail(ctx, accountEmail); err != nil {
		// If preference doesn't exist, that's okay - return success
		log.Printf("INFO: Shop preference not found for account_email: %s (may already be deleted)", accountEmail)
	}

	log.Printf("SUCCESS: Shop preference deleted")
	h.sendJSONResponse(w, http.StatusOK, map[string]interface{}{
		"success": true,
	})
}

