package shop_handler

import (
	"encoding/json"
	"fmt"
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
		log.Printf("ERROR: GetShopPreference called by non-customer user: %s (type: %s)", user.ID, user.UserType)
		h.sendErrorResponse(w, http.StatusForbidden, "Forbidden", "Only customers can have shop preferences")
		return
	}

	ctx := r.Context()

	// Get customer profile to get customer_id
	accountID, err := parseUserID(user.ID)
	if err != nil {
		log.Printf("ERROR: Invalid user ID format: %s", user.ID)
		h.sendErrorResponse(w, http.StatusBadRequest, "Invalid user ID", err.Error())
		return
	}

	customerProfile, err := h.customerProfileRepository.GetByAccountID(ctx, accountID)
	if err != nil {
		log.Printf("ERROR: Customer profile not found for account ID: %d - %v", accountID, err)
		h.sendErrorResponse(w, http.StatusNotFound, "Customer profile not found", err.Error())
		return
	}

	// Get shop preference
	preference, err := h.shopPreferenceRepository.GetByCustomerID(ctx, customerProfile.ID)
	if err != nil {
		// No preference found - return success with null shop
		log.Printf("INFO: No shop preference found for customer ID: %d", customerProfile.ID)
		h.sendJSONResponse(w, http.StatusOK, map[string]interface{}{
			"success": true,
			"shop":    nil,
		})
		return
	}

	log.Printf("SUCCESS: Shop preference retrieved for customer ID: %d, shop ID: %d", customerProfile.ID, preference.ShopID)
	h.sendJSONResponse(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"shop": map[string]interface{}{
			"id":        preference.ShopID,
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
		log.Printf("ERROR: SetShopPreference called by non-customer user: %s (type: %s)", user.ID, user.UserType)
		h.sendErrorResponse(w, http.StatusForbidden, "Forbidden", "Only customers can have shop preferences")
		return
	}

	// Parse request body
	var req struct {
		ShopID int64 `json:"shop_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("ERROR: Failed to decode request body: %v", err)
		h.sendErrorResponse(w, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

	if req.ShopID <= 0 {
		log.Printf("ERROR: Invalid shop_id: %d", req.ShopID)
		h.sendErrorResponse(w, http.StatusBadRequest, "Invalid shop_id", "shop_id must be a positive number")
		return
	}

	ctx := r.Context()

	// Get customer profile to get customer_id
	accountID, err := parseUserID(user.ID)
	if err != nil {
		log.Printf("ERROR: Invalid user ID format: %s", user.ID)
		h.sendErrorResponse(w, http.StatusBadRequest, "Invalid user ID", err.Error())
		return
	}

	customerProfile, err := h.customerProfileRepository.GetByAccountID(ctx, accountID)
	if err != nil {
		log.Printf("ERROR: Customer profile not found for account ID: %d - %v", accountID, err)
		h.sendErrorResponse(w, http.StatusNotFound, "Customer profile not found", err.Error())
		return
	}

	// Verify shop exists
	_, err = h.partnerProfileRepository.GetByID(ctx, req.ShopID)
	if err != nil {
		log.Printf("ERROR: Shop not found: %d - %v", req.ShopID, err)
		h.sendErrorResponse(w, http.StatusNotFound, "Shop not found", "The specified shop does not exist")
		return
	}

	// Save shop preference
	if err := h.shopPreferenceRepository.Upsert(ctx, customerProfile.ID, req.ShopID); err != nil {
		log.Printf("ERROR: Failed to save shop preference: %v", err)
		h.sendErrorResponse(w, http.StatusInternalServerError, "Failed to save shop preference", err.Error())
		return
	}

	log.Printf("SUCCESS: Shop preference saved for customer ID: %d, shop ID: %d", customerProfile.ID, req.ShopID)
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
		log.Printf("ERROR: DeleteShopPreference called by non-customer user: %s (type: %s)", user.ID, user.UserType)
		h.sendErrorResponse(w, http.StatusForbidden, "Forbidden", "Only customers can have shop preferences")
		return
	}

	ctx := r.Context()

	// Get customer profile to get customer_id
	accountID, err := parseUserID(user.ID)
	if err != nil {
		log.Printf("ERROR: Invalid user ID format: %s", user.ID)
		h.sendErrorResponse(w, http.StatusBadRequest, "Invalid user ID", err.Error())
		return
	}

	customerProfile, err := h.customerProfileRepository.GetByAccountID(ctx, accountID)
	if err != nil {
		log.Printf("ERROR: Customer profile not found for account ID: %d - %v", accountID, err)
		h.sendErrorResponse(w, http.StatusNotFound, "Customer profile not found", err.Error())
		return
	}

	// Delete shop preference
	if err := h.shopPreferenceRepository.DeleteByCustomerID(ctx, customerProfile.ID); err != nil {
		// If preference doesn't exist, that's okay - return success
		log.Printf("INFO: Shop preference not found for customer ID: %d (may already be deleted)", customerProfile.ID)
	}

	log.Printf("SUCCESS: Shop preference deleted for customer ID: %d", customerProfile.ID)
	h.sendJSONResponse(w, http.StatusOK, map[string]interface{}{
		"success": true,
	})
}

// parseUserID converts string user ID to int64
func parseUserID(userIDStr string) (int64, error) {
	var accountID int64
	_, err := fmt.Sscanf(userIDStr, "%d", &accountID)
	if err != nil {
		return 0, fmt.Errorf("invalid user ID format: %s", userIDStr)
	}
	return accountID, nil
}
