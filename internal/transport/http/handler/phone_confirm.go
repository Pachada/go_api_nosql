package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-api-nosql/internal/application/auth"
	"github.com/go-api-nosql/internal/transport/http/middleware"
)

// PhoneConfirmHandler handles phone confirmation flow endpoints.
type PhoneConfirmHandler struct {
	svc auth.PhoneConfirmationService
}

func NewPhoneConfirmHandler(svc auth.PhoneConfirmationService) *PhoneConfirmHandler {
	return &PhoneConfirmHandler{svc: svc}
}

func (h *PhoneConfirmHandler) Action(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.ClaimsFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	switch chi.URLParam(r, "action") {
	case "request":
		if err := h.svc.RequestPhoneConfirmation(r.Context(), claims.UserID); err != nil {
			httpError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, MessageEnvelope{Message: "confirmation SMS sent"})
	case "validate-code":
		var body struct {
			OTP string `json:"otp"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if err := h.svc.ValidatePhoneOTP(r.Context(), claims.UserID, body.OTP); err != nil {
			httpError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, MessageEnvelope{Message: "phone confirmed"})
	default:
		writeError(w, http.StatusBadRequest, "unknown action")
	}
}
