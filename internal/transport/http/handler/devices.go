package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-api-nosql/internal/application/device"
	"github.com/go-api-nosql/internal/domain"
	"github.com/go-api-nosql/internal/transport/http/middleware"
	"github.com/go-chi/chi/v5"
)

// DeviceHandler handles device endpoints.
type DeviceHandler struct {
	svc device.Service
}

func NewDeviceHandler(svc device.Service) *DeviceHandler { return &DeviceHandler{svc: svc} }

func (h *DeviceHandler) List(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.ClaimsFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	devices, err := h.svc.List(r.Context(), claims.UserID)
	if err != nil {
		httpError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, devices)
}

func (h *DeviceHandler) Get(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.ClaimsFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	d, err := h.svc.Get(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		httpError(w, err)
		return
	}
	if d.UserID != claims.UserID && claims.Role != domain.RoleAdmin {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	writeJSON(w, http.StatusOK, d)
}

func (h *DeviceHandler) Update(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.ClaimsFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	deviceID := chi.URLParam(r, "id")
	d, err := h.svc.Get(r.Context(), deviceID)
	if err != nil {
		httpError(w, err)
		return
	}
	if d.UserID != claims.UserID && claims.Role != domain.RoleAdmin {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	var req domain.UpdateDeviceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	updated, err := h.svc.Update(r.Context(), deviceID, req)
	if err != nil {
		httpError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (h *DeviceHandler) Delete(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.ClaimsFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	deviceID := chi.URLParam(r, "id")
	d, err := h.svc.Get(r.Context(), deviceID)
	if err != nil {
		httpError(w, err)
		return
	}
	if d.UserID != claims.UserID && claims.Role != domain.RoleAdmin {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	if err := h.svc.Delete(r.Context(), deviceID); err != nil {
		httpError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, MessageEnvelope{Message: "device deleted"})
}

func (h *DeviceHandler) CheckVersion(w http.ResponseWriter, r *http.Request) {
	var body struct {
		DeviceVersion float64 `json:"device_version"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	claims, ok := middleware.ClaimsFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	upToDate, err := h.svc.CheckVersion(r.Context(), claims.SessionID, body.DeviceVersion)
	if err != nil {
		httpError(w, err)
		return
	}
	if !upToDate {
		writeJSON(w, http.StatusConflict, MessageEnvelope{Message: "update required"})
		return
	}
	writeJSON(w, http.StatusOK, MessageEnvelope{Message: "up to date"})
}
