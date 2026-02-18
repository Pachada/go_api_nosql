package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-api-nosql/internal/application/auth"
	"github.com/go-api-nosql/internal/transport/http/middleware"
)

// EmailConfirmHandler handles email confirmation flow endpoints.
type EmailConfirmHandler struct {
	svc auth.Service
}

func NewEmailConfirmHandler(svc auth.Service) *EmailConfirmHandler {
	return &EmailConfirmHandler{svc: svc}
}

func (h *EmailConfirmHandler) Action(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.ClaimsFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	switch chi.URLParam(r, "action") {
	case "request":
		if err := h.svc.RequestEmailConfirmation(r.Context(), claims.UserID); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, MessageEnvelope{Message: "confirmation email sent"})
	case "validate-code":
		var body struct {
			Token string `json:"token"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if err := h.svc.ValidateEmailToken(r.Context(), claims.UserID, body.Token); err != nil {
			writeError(w, http.StatusUnauthorized, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, MessageEnvelope{Message: "email confirmed"})
	default:
		writeError(w, http.StatusBadRequest, "unknown action")
	}
}

