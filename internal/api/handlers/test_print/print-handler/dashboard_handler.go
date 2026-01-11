package print_handler

import (
	"log"
	"net/http"
	"strconv"

	"print-pro-backend/internal/models"
)

// DashboardOverview represents the dashboard statistics
type DashboardOverview struct {
	TotalOrders   int     `json:"total_orders"`
	PendingOrders int     `json:"pending_orders"`
	Completed     int     `json:"completed"`
	TotalRevenue  float64 `json:"total_revenue"` // In rupees
}

// GetDashboardOverview handles GET /api/test-print/dashboard/overview
// Returns dashboard statistics for the authenticated partner
func (h *PrintHandler) GetDashboardOverview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.sendErrorResponse(w, http.StatusMethodNotAllowed, "Only GET method is allowed", "Method not allowed")
		return
	}

	// Get authenticated user from context
	user := r.Context().Value("user")
	if user == nil {
		log.Printf("ERROR: No user in context for dashboard overview")
		h.sendErrorResponse(w, http.StatusUnauthorized, "Authentication required", "Unauthorized")
		return
	}

	authUser, ok := user.(*models.User)
	if !ok {
		log.Printf("ERROR: Invalid user type in context")
		h.sendErrorResponse(w, http.StatusUnauthorized, "Invalid authentication", "Unauthorized")
		return
	}

	// Only partners can access dashboard
	if authUser.UserType != "partner" {
		log.Printf("ERROR: Non-partner user attempting to access dashboard - UserType: %s", authUser.UserType)
		h.sendErrorResponse(w, http.StatusForbidden, "Dashboard is only available for partners", "Forbidden")
		return
	}

	ctx := r.Context()

	// Get partner profile to get partner_id
	accountID, err := strconv.ParseInt(authUser.ID, 10, 64)
	if err != nil {
		log.Printf("ERROR: Invalid account ID: %s - %v", authUser.ID, err)
		h.sendErrorResponse(w, http.StatusBadRequest, "Invalid account ID", "Invalid account")
		return
	}

	partnerProfile, err := h.partnerProfileRepo.GetByAccountID(ctx, accountID)
	if err != nil {
		log.Printf("ERROR: Partner profile not found for account_id %d - %v", accountID, err)
		h.sendErrorResponse(w, http.StatusNotFound, "Partner profile not found", "Partner not found")
		return
	}

	partnerID := partnerProfile.ID
	log.Printf("INFO: Fetching dashboard overview for partner_id: %d (shop: %s)", partnerID, partnerProfile.ShopName)

	// Query database for statistics
	totalOrders, pendingOrders, completed, totalRevenue, err := h.printJobRepo.GetDashboardStats(ctx, partnerID)
	if err != nil {
		log.Printf("ERROR: Failed to get dashboard stats for partner_id %d - %v", partnerID, err)
		h.sendErrorResponse(w, http.StatusInternalServerError, "Failed to retrieve dashboard statistics", "Internal error")
		return
	}

	overview := &DashboardOverview{
		TotalOrders:   totalOrders,
		PendingOrders: pendingOrders,
		Completed:     completed,
		TotalRevenue:  totalRevenue,
	}

	log.Printf("SUCCESS: Dashboard overview retrieved - Total Orders: %d, Pending: %d, Completed: %d, Revenue: ₹%.2f",
		overview.TotalOrders, overview.PendingOrders, overview.Completed, overview.TotalRevenue)

	h.sendJSONResponse(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    overview,
	})
}
