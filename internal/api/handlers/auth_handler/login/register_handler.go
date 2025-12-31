package login

import (
	"log"
	"net/http"
)

// RegisterHandler handles user registration (legacy - deprecated)
type RegisterHandler struct {
	sendErrorResponse func(http.ResponseWriter, int, string, string)
}

// NewRegisterHandler creates a new RegisterHandler instance
func NewRegisterHandler(
	sendErrorResponse func(http.ResponseWriter, int, string, string),
) *RegisterHandler {
	return &RegisterHandler{
		sendErrorResponse: sendErrorResponse,
	}
}

// HandleRegister handles user registration (legacy - deprecated)
func (h *RegisterHandler) HandleRegister(w http.ResponseWriter, r *http.Request) {
	log.Printf("Register endpoint hit (DEPRECATED) - Method: %s, URL: %s", r.Method, r.URL.Path)
	h.sendErrorResponse(w, http.StatusGone, "Deprecated", "This endpoint is deprecated. Please use /api/auth/register/partner or /api/auth/register/customer.")
}

