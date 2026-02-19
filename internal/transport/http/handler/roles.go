package handler

import (
	"net/http"

	"github.com/go-api-nosql/internal/domain"
)

// ListRoles returns the available role names. Roles are not stored in the
// database — they are hardcoded constants used for RBAC.
func ListRoles(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, []string{domain.RoleAdmin, domain.RoleUser})
}
