package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-api-nosql/internal/application/session"
	"github.com/go-api-nosql/internal/transport/http/middleware"
)

// SessionHandler handles session endpoints.
type SessionHandler struct {
	svc session.Service
}

func NewSessionHandler(svc session.Service) *SessionHandler {
	return &SessionHandler{svc: svc}
}

func (h *SessionHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req session.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	result, err := h.svc.Login(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, AuthEnvelope{
		AccessToken:  result.Bearer,
		RefreshToken: result.RefreshToken,
		Session:      toSafeSession(result.Session),
		User:         toSafeUser(result.Session.User),
	})
}

func (h *SessionHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.RefreshToken == "" {
		writeError(w, http.StatusBadRequest, "refresh_token required")
		return
	}
	bearer, newToken, err := h.svc.Refresh(r.Context(), req.RefreshToken)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, AuthEnvelope{AccessToken: bearer, RefreshToken: newToken})
}

func (h *SessionHandler) GetCurrent(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.ClaimsFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	sess, err := h.svc.GetCurrent(r.Context(), claims.SessionID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, SessionEnvelope{Session: toSafeSession(sess), User: toSafeUser(sess.User)})
}

func (h *SessionHandler) Logout(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.ClaimsFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if err := h.svc.Logout(r.Context(), claims.SessionID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, MessageEnvelope{Message: "logged out"})
}

