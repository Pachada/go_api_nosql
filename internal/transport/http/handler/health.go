package handler

import (
"context"
"net/http"

"github.com/go-chi/chi/v5"
)

// dbPinger is satisfied by any type that can verify database connectivity.
type dbPinger interface {
Ping(ctx context.Context) error
}

// HealthHandler handles health-check endpoints.
type HealthHandler struct {
db dbPinger
}

func NewHealthHandler(db dbPinger) *HealthHandler { return &HealthHandler{db: db} }

func (h *HealthHandler) Ping(w http.ResponseWriter, r *http.Request) {
action := chi.URLParam(r, "action")
switch action {
case "ping":
writeJSON(w, http.StatusOK, MessageEnvelope{Message: "pong"})
case "ready":
if err := h.db.Ping(r.Context()); err != nil {
writeError(w, http.StatusServiceUnavailable, "database unavailable")
return
}
writeJSON(w, http.StatusOK, MessageEnvelope{Message: "ok"})
default:
writeError(w, http.StatusBadRequest, "unknown action")
}
}

func (h *HealthHandler) Test(w http.ResponseWriter, _ *http.Request) {
writeJSON(w, http.StatusOK, MessageEnvelope{Message: "ok"})
}
