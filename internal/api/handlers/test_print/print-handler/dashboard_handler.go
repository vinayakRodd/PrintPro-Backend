package print_handler

import (
	"log"
	"net/http"

	"print-pro-backend/internal/middleware/auth_middleware"
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
	authUser, ok := auth_middleware.GetUserFromContext(r)
	if !ok {
		log.Printf("ERROR: No user in context for dashboard overview")
		h.sendErrorResponse(w, http.StatusUnauthorized, "Authentication required", "Unauthorized")
		return
	}

	// Only partners can access dashboard
	if authUser.UserType != "partner" {
		log.Printf("ERROR: Non-partner user attempting to access dashboard - UserType: %s", authUser.UserType)
		h.sendErrorResponse(w, http.StatusForbidden, "Dashboard is only available for partners", "Forbidden")
		return
	}

	ctx := r.Context()

	// Get partner profile to get partner_email
	accountEmail := authUser.ID

	partnerProfile, err := h.partnerProfileRepo.GetByAccountEmail(ctx, accountEmail)
	if err != nil {
		log.Printf("ERROR: Partner profile not found - %v", err)
		h.sendErrorResponse(w, http.StatusNotFound, "Partner profile not found", "Partner not found")
		return
	}

	partnerEmail := partnerProfile.PartnerEmail
	log.Printf("INFO: Fetching dashboard overview for partner (shop: %s)", partnerProfile.ShopName)

	// Query database for statistics
	totalOrders, pendingOrders, completed, totalRevenue, err := h.printJobRepo.GetDashboardStats(ctx, partnerEmail)
	if err != nil {
		log.Printf("ERROR: Failed to get dashboard stats - %v", err)
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
