package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-api-nosql/internal/application/user"
	"github.com/go-api-nosql/internal/domain"
	"github.com/go-api-nosql/internal/transport/http/middleware"
)

// UserHandler handles user CRUD endpoints.
type UserHandler struct {
	svc user.Service
}

func NewUserHandler(svc user.Service) *UserHandler { return &UserHandler{svc: svc} }

func (h *UserHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req domain.CreateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	sess, bearer, refreshToken, err := h.svc.RegisterWithSession(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, AuthEnvelope{
		Bearer:       bearer,
		RefreshToken: refreshToken,
		Session:      toSafeSession(sess),
	})
}

func (h *UserHandler) List(w http.ResponseWriter, r *http.Request) {
	page, perPage := parsePagination(r)
	users, total, err := h.svc.List(r.Context(), page, perPage)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	maxPage := 1
	if perPage > 0 && total > 0 {
		maxPage = (total + perPage - 1) / perPage
	}
	safe := make([]*SafeUser, len(users))
	for i := range users {
		safe[i] = toSafeUser(&users[i])
	}
	writeJSON(w, http.StatusOK, PaginatedUsersEnvelope{
		MaxPage: maxPage, ActualPage: page, PerPage: perPage, Data: safe,
	})
}

func (h *UserHandler) Get(w http.ResponseWriter, r *http.Request) {
	u, err := h.svc.Get(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, toSafeUser(u))
}

func (h *UserHandler) Update(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.ClaimsFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	targetID := chi.URLParam(r, "id")
	if claims.UserID != targetID && claims.RoleID == "" {
		writeError(w, http.StatusUnauthorized, "cannot update another user")
		return
	}
	var req domain.UpdateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	u, err := h.svc.Update(r.Context(), targetID, req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, toSafeUser(u))
}

func (h *UserHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.Delete(r.Context(), chi.URLParam(r, "id")); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, MessageEnvelope{Message: "user deleted"})
}

func parsePagination(r *http.Request) (page, perPage int) {
	page, _ = strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	perPage, _ = strconv.Atoi(r.URL.Query().Get("per_page"))
	if perPage < 1 {
		perPage = 50
	}
	return
}

