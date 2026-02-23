package middleware

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-api-nosql/internal/config"
	jwtinfra "github.com/go-api-nosql/internal/infrastructure/jwt"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestProvider generates a fresh RSA key pair, writes them to temp files,
// and returns a *jwtinfra.Provider. The temp directory is cleaned up automatically
// by t.TempDir() when the test completes.
func newTestProvider(t *testing.T) *jwtinfra.Provider {
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

	cfg := &config.Config{
		JWTPrivateKeyPath: privPath,
		JWTPublicKeyPath:  pubPath,
		JWTExpiry:         24 * time.Hour,
	}
	p, err := jwtinfra.NewProvider(cfg)
	require.NoError(t, err)
	return p
}

func okHandler(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }

func TestAuth_MissingHeader(t *testing.T) {
	p := newTestProvider(t)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	Auth(p)(http.HandlerFunc(okHandler)).ServeHTTP(rr, req)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestAuth_BadToken(t *testing.T) {
	p := newTestProvider(t)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer not-a-real-token")
	rr := httptest.NewRecorder()
	Auth(p)(http.HandlerFunc(okHandler)).ServeHTTP(rr, req)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestAuth_ExpiredToken(t *testing.T) {
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	claims := &jwtinfra.Claims{
		UserID: "u1",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)), // already expired
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	signed, err := token.SignedString(privKey)
	require.NoError(t, err)

	p := newTestProvider(t) // different key pair — will fail verification

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+signed)
	rr := httptest.NewRecorder()
	Auth(p)(http.HandlerFunc(okHandler)).ServeHTTP(rr, req)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestAuth_ValidToken_InjectsClaims(t *testing.T) {
	p := newTestProvider(t)

	signed, err := p.Sign("u1", "dev1", "user", "sess1")
	require.NoError(t, err)

	var gotClaims *jwtinfra.Claims
	captureHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotClaims, _ = ClaimsFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+signed)
	rr := httptest.NewRecorder()
	Auth(p)(captureHandler).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	require.NotNil(t, gotClaims)
	assert.Equal(t, "u1", gotClaims.UserID)
	assert.Equal(t, "user", gotClaims.Role)
}
