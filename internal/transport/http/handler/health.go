package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// HealthHandler handles health-check and test endpoints.
type HealthHandler struct{}

func NewHealthHandler() *HealthHandler { return &HealthHandler{} }

func (h *HealthHandler) Ping(w http.ResponseWriter, r *http.Request) {
	action := chi.URLParam(r, "action")
	if action == "ping" {
		writeJSON(w, http.StatusOK, MessageEnvelope{Message: "pong"})
		return
	}
	writeError(w, http.StatusBadRequest, "unknown action")
}

func (h *HealthHandler) Test(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, MessageEnvelope{Message: "ok"})
}

