package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-api-nosql/internal/application/auth"
	"github.com/go-api-nosql/internal/pkg/validate"
	"github.com/go-api-nosql/internal/transport/http/middleware"
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
		bearer, refreshToken, sess, err := h.svc.ValidateOTP(r.Context(), req)
		if err != nil {
			httpError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, AuthEnvelope{AccessToken: bearer, RefreshToken: refreshToken, Session: toSafeSession(sess), User: toSafeUser(sess.User)})
	default:
		writeError(w, http.StatusBadRequest, "unknown action")
	}
}

func (h *PasswordRecoveryHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.ClaimsFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var req auth.ChangePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := validate.Struct(&req); err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if err := h.svc.ChangePassword(r.Context(), claims.UserID, req.NewPassword); err != nil {
		httpError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, MessageEnvelope{Message: "password changed"})
}

