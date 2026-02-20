package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	jwtinfra "github.com/go-api-nosql/internal/infrastructure/jwt"
	"github.com/stretchr/testify/assert"
)

func TestRequireRole_NoClaimsInContext(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	RequireRole("admin")(http.HandlerFunc(okHandler)).ServeHTTP(rr, req)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestRequireRole_WrongRole(t *testing.T) {
	claims := &jwtinfra.Claims{Role: "user"}
	ctx := context.WithValue(context.Background(), claimsKey, claims)
	req := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx)
	rr := httptest.NewRecorder()
	RequireRole("admin")(http.HandlerFunc(okHandler)).ServeHTTP(rr, req)
	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestRequireRole_CorrectRole(t *testing.T) {
	claims := &jwtinfra.Claims{Role: "admin"}
	ctx := context.WithValue(context.Background(), claimsKey, claims)
	req := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx)
	rr := httptest.NewRecorder()
	RequireRole("admin")(http.HandlerFunc(okHandler)).ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestRequireRole_MultipleAllowedRoles(t *testing.T) {
	claims := &jwtinfra.Claims{Role: "user"}
	ctx := context.WithValue(context.Background(), claimsKey, claims)
	req := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx)
	rr := httptest.NewRecorder()
	RequireRole("admin", "user")(http.HandlerFunc(okHandler)).ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}
