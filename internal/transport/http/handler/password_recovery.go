package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-api-nosql/internal/application/auth"
	"github.com/go-api-nosql/internal/pkg/validate"
	"github.com/go-chi/chi/v5"
)

// PasswordRecoveryHandler handles password recovery flow endpoints.
type PasswordRecoveryHandler struct {
	svc auth.PasswordRecoveryService
}

func NewPasswordRecoveryHandler(svc auth.PasswordRecoveryService) *PasswordRecoveryHandler {
	return &PasswordRecoveryHandler{svc: svc}
}

func (h *PasswordRecoveryHandler) Action(w http.ResponseWriter, r *http.Request) {
	switch chi.URLParam(r, "action") {
	case "request":
		var req auth.PasswordRecoveryRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if err := h.svc.RequestPasswordRecovery(r.Context(), req); err != nil {
			httpError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, MessageEnvelope{Message: "OTP sent"})
	case "validate-code":
		var req auth.ValidateOTPRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if err := validate.Struct(&req); err != nil {
			writeError(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		result, err := h.svc.ValidateOTP(r.Context(), req)
		if err != nil {
			httpError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, AuthEnvelope{AccessToken: result.Bearer, RefreshToken: result.RefreshToken, Session: toSafeSession(result.Session), User: toSafeUser(result.Session.User)})
	default:
		writeError(w, http.StatusBadRequest, "unknown action")
	}
}
