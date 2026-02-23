package handler

import (
	"net/http"

	"github.com/go-api-nosql/internal/application/notification"
	"github.com/go-api-nosql/internal/transport/http/middleware"
	"github.com/go-chi/chi/v5"
)

// NotificationHandler handles notification endpoints.
type NotificationHandler struct {
	svc notification.Service
}

func NewNotificationHandler(svc notification.Service) *NotificationHandler {
	return &NotificationHandler{svc: svc}
}

func (h *NotificationHandler) ListUnread(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.ClaimsFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	notifications, err := h.svc.ListUnread(r.Context(), claims.UserID)
	if err != nil {
		httpError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, notifications)
}

func (h *NotificationHandler) MarkAsRead(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.ClaimsFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	n, err := h.svc.MarkAsRead(r.Context(), chi.URLParam(r, "id"), claims.UserID)
	if err != nil {
		httpError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, n)
}
