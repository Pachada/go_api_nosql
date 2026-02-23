package handler

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-api-nosql/internal/config"
	"github.com/go-api-nosql/internal/domain"
	jwtinfra "github.com/go-api-nosql/internal/infrastructure/jwt"
	"github.com/go-api-nosql/internal/transport/http/middleware"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// --- mock ---

type mockUserSvc struct{ mock.Mock }

func (m *mockUserSvc) Register(ctx context.Context, req domain.CreateUserRequest) (*domain.User, error) {
	args := m.Called(ctx, req)
	if u, _ := args.Get(0).(*domain.User); u != nil {
		return u, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockUserSvc) RegisterWithSession(ctx context.Context, req domain.CreateUserRequest) (*domain.Session, string, string, error) {
	args := m.Called(ctx, req)
	if s, _ := args.Get(0).(*domain.Session); s != nil {
		return s, args.String(1), args.String(2), args.Error(3)
	}
	return nil, "", "", args.Error(3)
}

func (m *mockUserSvc) List(ctx context.Context, limit int, cursor string) ([]domain.User, string, error) {
	args := m.Called(ctx, limit, cursor)
	return args.Get(0).([]domain.User), args.String(1), args.Error(2)
}

func (m *mockUserSvc) Get(ctx context.Context, userID string) (*domain.User, error) {
	args := m.Called(ctx, userID)
	if u, _ := args.Get(0).(*domain.User); u != nil {
		return u, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockUserSvc) Update(ctx context.Context, userID string, req domain.UpdateUserRequest) (*domain.User, error) {
	args := m.Called(ctx, userID, req)
	if u, _ := args.Get(0).(*domain.User); u != nil {
		return u, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockUserSvc) Delete(ctx context.Context, userID string) error {
	return m.Called(ctx, userID).Error(0)
}

func (m *mockUserSvc) ChangePassword(ctx context.Context, userID, currentPassword, newPassword string) error {
	return m.Called(ctx, userID, currentPassword, newPassword).Error(0)
}

// --- helpers ---

// newTestJWTProvider generates a fresh RSA key pair and returns a *jwtinfra.Provider.
func newTestJWTProvider(t *testing.T) *jwtinfra.Provider {
	t.Helper()
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	dir := t.TempDir()
	privPath := filepath.Join(dir, "private.pem")
	pubPath := filepath.Join(dir, "public.pem")

	privPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privKey)})
	require.NoError(t, os.WriteFile(privPath, privPEM, 0600))

	pubBytes, err := x509.MarshalPKIXPublicKey(&privKey.PublicKey)
	require.NoError(t, err)
	pubPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubBytes})
	require.NoError(t, os.WriteFile(pubPath, pubPEM, 0600))

	p, err := jwtinfra.NewProvider(&config.Config{
		JWTPrivateKeyPath: privPath,
		JWTPublicKeyPath:  pubPath,
		JWTExpiry:         24 * time.Hour,
	})
	require.NoError(t, err)
	return p
}

// bearerReq builds a request with a signed Bearer token for the given userID and role.
func bearerReq(t *testing.T, p *jwtinfra.Provider, method, target, userID, role string, body []byte) *http.Request {
	t.Helper()
	token, err := p.Sign(userID, "dev1", role, "sess1")
	require.NoError(t, err)
	var r *http.Request
	if body != nil {
		r = httptest.NewRequest(method, target, bytes.NewReader(body))
	} else {
		r = httptest.NewRequest(method, target, nil)
	}
	r.Header.Set("Authorization", "Bearer "+token)
	return r
}

// withChiID injects a chi URL param "id" into the request context.
func withChiID(r *http.Request, id string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", id)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

// serveAuthed wraps the handler with middleware.Auth before serving.
func serveAuthed(p *jwtinfra.Provider, h http.Handler, w http.ResponseWriter, r *http.Request) {
	middleware.Auth(p)(h).ServeHTTP(w, r)
}

// --- Register tests ---

func TestRegister_InvalidBody(t *testing.T) {
	svc := &mockUserSvc{}
	h := NewUserHandler(svc)
	r := httptest.NewRequest(http.MethodPost, "/v1/users", bytes.NewBufferString("not-json"))
	rr := httptest.NewRecorder()
	h.Register(rr, r)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestRegister_ValidationFailure(t *testing.T) {
	svc := &mockUserSvc{}
	h := NewUserHandler(svc)
	body, _ := json.Marshal(domain.CreateUserRequest{Username: "alice"}) // missing required fields
	r := httptest.NewRequest(http.MethodPost, "/v1/users", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	h.Register(rr, r)
	assert.Equal(t, http.StatusUnprocessableEntity, rr.Code)
}

func TestRegister_ServiceConflict(t *testing.T) {
	svc := &mockUserSvc{}
	svc.On("RegisterWithSession", mock.Anything, mock.Anything).Return(nil, "", "", domain.ErrConflict)
	h := NewUserHandler(svc)
	body, _ := json.Marshal(domain.CreateUserRequest{
		Username: "alice", Password: "secret123", Email: "alice@example.com",
		FirstName: "Alice", LastName: "Smith",
	})
	r := httptest.NewRequest(http.MethodPost, "/v1/users", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	h.Register(rr, r)
	assert.Equal(t, http.StatusConflict, rr.Code)
	svc.AssertExpectations(t)
}

func TestRegister_HappyPath(t *testing.T) {
	svc := &mockUserSvc{}
	sess := &domain.Session{SessionID: "s1", UserID: "u1", User: &domain.User{UserID: "u1", Username: "alice"}}
	svc.On("RegisterWithSession", mock.Anything, mock.Anything).Return(sess, "access-token", "refresh-token", nil)
	h := NewUserHandler(svc)
	body, _ := json.Marshal(domain.CreateUserRequest{
		Username: "alice", Password: "secret123", Email: "alice@example.com",
		FirstName: "Alice", LastName: "Smith",
	})
	r := httptest.NewRequest(http.MethodPost, "/v1/users", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	h.Register(rr, r)
	assert.Equal(t, http.StatusCreated, rr.Code)
	var resp AuthEnvelope
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Equal(t, "access-token", resp.AccessToken)
	assert.Equal(t, "alice", resp.User.Username)
	svc.AssertExpectations(t)
}

// --- Get tests ---

func TestGet_MissingClaims(t *testing.T) {
	svc := &mockUserSvc{}
	h := NewUserHandler(svc)
	r := withChiID(httptest.NewRequest(http.MethodGet, "/v1/users/u1", nil), "u1")
	rr := httptest.NewRecorder()
	h.Get(rr, r) // called directly, no claims in context
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestGet_Owner_SeesFullUser(t *testing.T) {
	p := newTestJWTProvider(t)
	svc := &mockUserSvc{}
	u := &domain.User{UserID: "u1", Username: "alice", Email: "alice@example.com", Role: domain.RoleUser}
	svc.On("Get", mock.Anything, "u1").Return(u, nil)
	h := NewUserHandler(svc)

	r := bearerReq(t, p, http.MethodGet, "/v1/users/u1", "u1", domain.RoleUser, nil)
	r = withChiID(r, "u1")
	rr := httptest.NewRecorder()
	serveAuthed(p, http.HandlerFunc(h.Get), rr, r)

	assert.Equal(t, http.StatusOK, rr.Code)
	var resp SafeUser
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Equal(t, "alice@example.com", resp.Email)
	svc.AssertExpectations(t)
}

func TestGet_Admin_SeesFullUser(t *testing.T) {
	p := newTestJWTProvider(t)
	svc := &mockUserSvc{}
	u := &domain.User{UserID: "u2", Username: "bob", Email: "bob@example.com", Role: domain.RoleUser}
	svc.On("Get", mock.Anything, "u2").Return(u, nil)
	h := NewUserHandler(svc)

	r := bearerReq(t, p, http.MethodGet, "/v1/users/u2", "admin1", domain.RoleAdmin, nil)
	r = withChiID(r, "u2")
	rr := httptest.NewRecorder()
	serveAuthed(p, http.HandlerFunc(h.Get), rr, r)

	assert.Equal(t, http.StatusOK, rr.Code)
	var resp SafeUser
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Equal(t, "bob@example.com", resp.Email)
	svc.AssertExpectations(t)
}

func TestGet_OtherUser_SeesPublicOnly(t *testing.T) {
	p := newTestJWTProvider(t)
	svc := &mockUserSvc{}
	u := &domain.User{UserID: "u2", Username: "bob", Email: "bob@example.com", Role: domain.RoleUser}
	svc.On("Get", mock.Anything, "u2").Return(u, nil)
	h := NewUserHandler(svc)

	r := bearerReq(t, p, http.MethodGet, "/v1/users/u2", "u1", domain.RoleUser, nil) // u1 viewing u2
	r = withChiID(r, "u2")
	rr := httptest.NewRecorder()
	serveAuthed(p, http.HandlerFunc(h.Get), rr, r)

	assert.Equal(t, http.StatusOK, rr.Code)
	// PublicUser has no email field — verify email is absent in response
	var resp map[string]interface{}
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	_, hasEmail := resp["email"]
	assert.False(t, hasEmail, "other users should not see email in response")
	svc.AssertExpectations(t)
}

// --- Update tests ---

func TestUpdate_MissingClaims(t *testing.T) {
	svc := &mockUserSvc{}
	h := NewUserHandler(svc)
	r := withChiID(httptest.NewRequest(http.MethodPut, "/v1/users/u1", nil), "u1")
	rr := httptest.NewRecorder()
	h.Update(rr, r)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestUpdate_NotOwnerOrAdmin(t *testing.T) {
	p := newTestJWTProvider(t)
	svc := &mockUserSvc{}
	h := NewUserHandler(svc)

	r := bearerReq(t, p, http.MethodPut, "/v1/users/u2", "u1", domain.RoleUser, nil)
	r = withChiID(r, "u2") // u1 trying to update u2
	rr := httptest.NewRecorder()
	serveAuthed(p, http.HandlerFunc(h.Update), rr, r)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestUpdate_NonAdmin_CannotSetRole(t *testing.T) {
	p := newTestJWTProvider(t)
	svc := &mockUserSvc{}
	h := NewUserHandler(svc)
	role := domain.RoleAdmin
	body, _ := json.Marshal(domain.UpdateUserRequest{Role: &role})

	r := bearerReq(t, p, http.MethodPut, "/v1/users/u1", "u1", domain.RoleUser, body)
	r = withChiID(r, "u1") // self-update but with role field
	rr := httptest.NewRecorder()
	serveAuthed(p, http.HandlerFunc(h.Update), rr, r)

	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestUpdate_HappyPath_SelfUpdate(t *testing.T) {
	p := newTestJWTProvider(t)
	svc := &mockUserSvc{}
	updated := &domain.User{UserID: "u1", Username: "alice2", Email: "alice@example.com"}
	svc.On("Update", mock.Anything, "u1", mock.Anything).Return(updated, nil)
	h := NewUserHandler(svc)
	newName := "alice2"
	body, _ := json.Marshal(domain.UpdateUserRequest{Username: &newName})

	r := bearerReq(t, p, http.MethodPut, "/v1/users/u1", "u1", domain.RoleUser, body)
	r = withChiID(r, "u1")
	rr := httptest.NewRecorder()
	serveAuthed(p, http.HandlerFunc(h.Update), rr, r)

	assert.Equal(t, http.StatusOK, rr.Code)
	var resp SafeUser
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Equal(t, "alice2", resp.Username)
	svc.AssertExpectations(t)
}

func TestUpdate_Admin_CanSetRole(t *testing.T) {
	p := newTestJWTProvider(t)
	svc := &mockUserSvc{}
	updated := &domain.User{UserID: "u2", Username: "bob", Role: domain.RoleAdmin}
	svc.On("Update", mock.Anything, "u2", mock.Anything).Return(updated, nil)
	h := NewUserHandler(svc)
	newRole := domain.RoleAdmin
	body, _ := json.Marshal(domain.UpdateUserRequest{Role: &newRole})

	r := bearerReq(t, p, http.MethodPut, "/v1/users/u2", "admin1", domain.RoleAdmin, body)
	r = withChiID(r, "u2")
	rr := httptest.NewRecorder()
	serveAuthed(p, http.HandlerFunc(h.Update), rr, r)

	assert.Equal(t, http.StatusOK, rr.Code)
	svc.AssertExpectations(t)
}

// --- Delete tests ---

func TestDelete_MissingClaims(t *testing.T) {
	svc := &mockUserSvc{}
	h := NewUserHandler(svc)
	r := withChiID(httptest.NewRequest(http.MethodDelete, "/v1/users/u1", nil), "u1")
	rr := httptest.NewRecorder()
	h.Delete(rr, r)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestDelete_NotOwnerOrAdmin(t *testing.T) {
	p := newTestJWTProvider(t)
	svc := &mockUserSvc{}
	h := NewUserHandler(svc)

	r := bearerReq(t, p, http.MethodDelete, "/v1/users/u2", "u1", domain.RoleUser, nil)
	r = withChiID(r, "u2") // u1 trying to delete u2
	rr := httptest.NewRecorder()
	serveAuthed(p, http.HandlerFunc(h.Delete), rr, r)

	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestDelete_HappyPath_SelfDelete(t *testing.T) {
	p := newTestJWTProvider(t)
	svc := &mockUserSvc{}
	svc.On("Delete", mock.Anything, "u1").Return(nil)
	h := NewUserHandler(svc)

	r := bearerReq(t, p, http.MethodDelete, "/v1/users/u1", "u1", domain.RoleUser, nil)
	r = withChiID(r, "u1")
	rr := httptest.NewRecorder()
	serveAuthed(p, http.HandlerFunc(h.Delete), rr, r)

	assert.Equal(t, http.StatusOK, rr.Code)
	svc.AssertExpectations(t)
}

func TestDelete_Admin_DeletesOtherUser(t *testing.T) {
	p := newTestJWTProvider(t)
	svc := &mockUserSvc{}
	svc.On("Delete", mock.Anything, "u2").Return(nil)
	h := NewUserHandler(svc)

	r := bearerReq(t, p, http.MethodDelete, "/v1/users/u2", "admin1", domain.RoleAdmin, nil)
	r = withChiID(r, "u2")
	rr := httptest.NewRecorder()
	serveAuthed(p, http.HandlerFunc(h.Delete), rr, r)

	assert.Equal(t, http.StatusOK, rr.Code)
	svc.AssertExpectations(t)
}

// --- ChangePassword tests ---

func TestChangePassword_MissingClaims(t *testing.T) {
	svc := &mockUserSvc{}
	h := NewUserHandler(svc)
	r := httptest.NewRequest(http.MethodPost, "/v1/users/me/password", nil)
	rr := httptest.NewRecorder()
	h.ChangePassword(rr, r)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestChangePassword_InvalidBody(t *testing.T) {
	p := newTestJWTProvider(t)
	svc := &mockUserSvc{}
	h := NewUserHandler(svc)
	body, _ := json.Marshal(map[string]string{"current_password": "old"}) // missing new_password

	r := bearerReq(t, p, http.MethodPost, "/v1/users/me/password", "u1", domain.RoleUser, body)
	rr := httptest.NewRecorder()
	serveAuthed(p, http.HandlerFunc(h.ChangePassword), rr, r)

	assert.Equal(t, http.StatusUnprocessableEntity, rr.Code)
}

func TestChangePassword_HappyPath(t *testing.T) {
	p := newTestJWTProvider(t)
	svc := &mockUserSvc{}
	svc.On("ChangePassword", mock.Anything, "u1", "oldpass1", "newpass123").Return(nil)
	h := NewUserHandler(svc)
	body, _ := json.Marshal(ChangePasswordRequest{CurrentPassword: "oldpass1", NewPassword: "newpass123"})

	r := bearerReq(t, p, http.MethodPost, "/v1/users/me/password", "u1", domain.RoleUser, body)
	rr := httptest.NewRecorder()
	serveAuthed(p, http.HandlerFunc(h.ChangePassword), rr, r)

	assert.Equal(t, http.StatusOK, rr.Code)
	svc.AssertExpectations(t)
}
