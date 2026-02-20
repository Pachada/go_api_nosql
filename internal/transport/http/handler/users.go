package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-api-nosql/internal/application/user"
	"github.com/go-api-nosql/internal/domain"
	"github.com/go-api-nosql/internal/pkg/validate"
	"github.com/go-api-nosql/internal/transport/http/middleware"
	"github.com/go-chi/chi/v5"
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
	if err := validate.Struct(&req); err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	sess, bearer, refreshToken, err := h.svc.RegisterWithSession(r.Context(), req)
	if err != nil {
		httpError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, AuthEnvelope{
		AccessToken:  bearer,
		RefreshToken: refreshToken,
		Session:      toSafeSession(sess),
		User:         toSafeUser(sess.User),
	})
}

func (h *UserHandler) List(w http.ResponseWriter, r *http.Request) {
	limit, cursor := parseCursorPagination(r)
	users, nextCursor, err := h.svc.List(r.Context(), limit, cursor)
	if err != nil {
		httpError(w, err)
		return
	}
	safe := make([]*SafeUser, len(users))
	for i := range users {
		safe[i] = toSafeUser(&users[i])
	}
	writeJSON(w, http.StatusOK, CursorUsersEnvelope{
		Data:       safe,
		Returned:   len(safe),
		NextCursor: nextCursor,
	})
}

func (h *UserHandler) Get(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.ClaimsFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	u, err := h.svc.Get(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		httpError(w, err)
		return
	}
	if claims.UserID == u.UserID || claims.Role == domain.RoleAdmin {
		writeJSON(w, http.StatusOK, toSafeUser(u))
		return
	}
	writeJSON(w, http.StatusOK, toPublicUser(u))
}

func (h *UserHandler) Update(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.ClaimsFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	targetID := chi.URLParam(r, "id")
	if claims.UserID != targetID && claims.Role != domain.RoleAdmin {
		writeError(w, http.StatusUnauthorized, "cannot update another user")
		return
	}
	var req domain.UpdateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := validate.Struct(&req); err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if claims.Role != domain.RoleAdmin {
		if req.Role != nil || req.Enable != nil {
			writeError(w, http.StatusForbidden, "cannot set role or enable as non-admin")
			return
		}
	}
	u, err := h.svc.Update(r.Context(), targetID, req)
	if err != nil {
		httpError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toSafeUser(u))
}

func (h *UserHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.Delete(r.Context(), chi.URLParam(r, "id")); err != nil {
		httpError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, MessageEnvelope{Message: "deleted"})
}

func parseCursorPagination(r *http.Request) (limit int, cursor string) {
	limit, _ = strconv.Atoi(r.URL.Query().Get("limit"))
	if limit < 1 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}
	cursor = r.URL.Query().Get("cursor")
	return
}
