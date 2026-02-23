package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-api-nosql/internal/application/status"
	"github.com/go-api-nosql/internal/domain"
	"github.com/go-chi/chi/v5"
)

// StatusHandler handles status endpoints.
type StatusHandler struct {
	svc status.Service
}

func NewStatusHandler(svc status.Service) *StatusHandler { return &StatusHandler{svc: svc} }

func (h *StatusHandler) List(w http.ResponseWriter, r *http.Request) {
	statuses, err := h.svc.List(r.Context())
	if err != nil {
		httpError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, statuses)
}

func (h *StatusHandler) Create(w http.ResponseWriter, r *http.Request) {
	var input domain.StatusInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	created, err := h.svc.Create(r.Context(), input)
	if err != nil {
		httpError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (h *StatusHandler) Get(w http.ResponseWriter, r *http.Request) {
	st, err := h.svc.Get(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		httpError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (h *StatusHandler) Update(w http.ResponseWriter, r *http.Request) {
	var input domain.StatusInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	updated, err := h.svc.Update(r.Context(), chi.URLParam(r, "id"), input)
	if err != nil {
		httpError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

// Delete is a hard delete (no soft delete for statuses).
func (h *StatusHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.Delete(r.Context(), chi.URLParam(r, "id")); err != nil {
		httpError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, MessageEnvelope{Message: "status deleted"})
}
