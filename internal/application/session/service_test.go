package session

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/go-api-nosql/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// --- mocks ---

type mockUserStore struct{ mock.Mock }

func (m *mockUserStore) GetByUsername(ctx context.Context, username string) (*domain.User, error) {
	args := m.Called(ctx, username)
	if u, _ := args.Get(0).(*domain.User); u != nil {
		return u, args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *mockUserStore) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	args := m.Called(ctx, email)
	if u, _ := args.Get(0).(*domain.User); u != nil {
		return u, args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *mockUserStore) Get(ctx context.Context, userID string) (*domain.User, error) {
	args := m.Called(ctx, userID)
	if u, _ := args.Get(0).(*domain.User); u != nil {
		return u, args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *mockUserStore) Put(ctx context.Context, u *domain.User) error {
	return m.Called(ctx, u).Error(0)
}
func (m *mockUserStore) Update(ctx context.Context, userID string, updates map[string]interface{}) error {
	return m.Called(ctx, userID, updates).Error(0)
}

type mockSessionStore struct{ mock.Mock }

func (m *mockSessionStore) Put(ctx context.Context, s *domain.Session) error {
	return m.Called(ctx, s).Error(0)
}
func (m *mockSessionStore) Get(ctx context.Context, sessionID string) (*domain.Session, error) {
	args := m.Called(ctx, sessionID)
	if s, _ := args.Get(0).(*domain.Session); s != nil {
		return s, args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *mockSessionStore) GetByRefreshToken(ctx context.Context, token string) (*domain.Session, error) {
	args := m.Called(ctx, token)
	if s, _ := args.Get(0).(*domain.Session); s != nil {
		return s, args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *mockSessionStore) RotateRefreshToken(ctx context.Context, sessionID, newToken string, newExpiry int64) error {
	return m.Called(ctx, sessionID, newToken, newExpiry).Error(0)
}
func (m *mockSessionStore) Update(ctx context.Context, sessionID string, updates map[string]interface{}) error {
	return m.Called(ctx, sessionID, updates).Error(0)
}

type mockDeviceStore struct{ mock.Mock }

func (m *mockDeviceStore) GetByUUID(ctx context.Context, uuid string) (*domain.Device, error) {
	args := m.Called(ctx, uuid)
	if d, _ := args.Get(0).(*domain.Device); d != nil {
		return d, args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *mockDeviceStore) Put(ctx context.Context, d *domain.Device) error {
	return m.Called(ctx, d).Error(0)
}

type mockJWTSigner struct{ mock.Mock }

func (m *mockJWTSigner) Sign(userID, deviceID, role, sessionID string) (string, error) {
	args := m.Called(userID, deviceID, role, sessionID)
	return args.String(0), args.Error(1)
}

type mockGoogleVerifier struct{ mock.Mock }

func (m *mockGoogleVerifier) Verify(ctx context.Context, token string) (*GooglePayload, error) {
	args := m.Called(ctx, token)
	if p, _ := args.Get(0).(*GooglePayload); p != nil {
		return p, args.Error(1)
	}
	return nil, args.Error(1)
}

// --- helpers ---

func newSvc(us *mockUserStore, ss *mockSessionStore, ds *mockDeviceStore, jwt *mockJWTSigner, gv *mockGoogleVerifier) Service {
	return NewService(ServiceDeps{
		UserRepo:        us,
		SessionRepo:     ss,
		DeviceRepo:      ds,
		JWTProvider:     jwt,
		GoogleVerifier:  gv,
		RefreshTokenDur: 24 * time.Hour,
	})
}

func validPayload() *GooglePayload {
	return &GooglePayload{
		Sub:           "google-sub-123",
		Email:         "alice@gmail.com",
		EmailVerified: true,
		FirstName:     "Alice",
		LastName:      "Smith",
	}
}

func existingUser() *domain.User {
	return &domain.User{
		UserID:    "user-123",
		Username:  "alice",
		Email:     "alice@gmail.com",
		Role:      domain.RoleUser,
		Enable:    1,
		GoogleSub: "google-sub-123",
	}
}

func stubDevice(ds *mockDeviceStore) *domain.Device {
	dev := &domain.Device{DeviceID: "dev-1", UUID: "uuid-1", UserID: "user-123", Enable: true}
	ds.On("GetByUUID", mock.Anything, mock.Anything).Return(nil, domain.ErrNotFound)
	ds.On("Put", mock.Anything, mock.AnythingOfType("*domain.Device")).Return(nil)
	return dev
}

// --- LoginWithGoogle tests ---

func TestLoginWithGoogle_NewUser(t *testing.T) {
	us, ss, ds, jwt, gv := &mockUserStore{}, &mockSessionStore{}, &mockDeviceStore{}, &mockJWTSigner{}, &mockGoogleVerifier{}

	gv.On("Verify", mock.Anything, "tok").Return(validPayload(), nil)
	us.On("GetByEmail", mock.Anything, "alice@gmail.com").Return(nil, domain.ErrNotFound)
	us.On("GetByUsername", mock.Anything, "alice").Return(nil, domain.ErrNotFound)
	us.On("Put", mock.Anything, mock.AnythingOfType("*domain.User")).Return(nil)
	stubDevice(ds)
	ss.On("Put", mock.Anything, mock.AnythingOfType("*domain.Session")).Return(nil)
	jwt.On("Sign", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("bearer", nil)

	result, err := newSvc(us, ss, ds, jwt, gv).LoginWithGoogle(context.Background(), "tok", nil)

	require.NoError(t, err)
	assert.Equal(t, "bearer", result.Bearer)
	assert.NotEmpty(t, result.RefreshToken)
	assert.Equal(t, "alice", result.Session.User.Username)
	assert.Equal(t, domain.AuthProviderGoogle, result.Session.User.AuthProvider)
	assert.True(t, result.Session.User.EmailConfirmed)
}

func TestLoginWithGoogle_ExistingUser_SubMatches(t *testing.T) {
	us, ss, ds, jwt, gv := &mockUserStore{}, &mockSessionStore{}, &mockDeviceStore{}, &mockJWTSigner{}, &mockGoogleVerifier{}

	gv.On("Verify", mock.Anything, "tok").Return(validPayload(), nil)
	us.On("GetByEmail", mock.Anything, "alice@gmail.com").Return(existingUser(), nil)
	stubDevice(ds)
	ss.On("Put", mock.Anything, mock.AnythingOfType("*domain.Session")).Return(nil)
	jwt.On("Sign", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("bearer", nil)

	result, err := newSvc(us, ss, ds, jwt, gv).LoginWithGoogle(context.Background(), "tok", nil)

	require.NoError(t, err)
	assert.Equal(t, "bearer", result.Bearer)
}

func TestLoginWithGoogle_ExistingUser_FirstGoogleSignIn_AutoLinks(t *testing.T) {
	us, ss, ds, jwt, gv := &mockUserStore{}, &mockSessionStore{}, &mockDeviceStore{}, &mockJWTSigner{}, &mockGoogleVerifier{}

	user := existingUser()
	user.GoogleSub = ""
	user.PasswordHash = "$2a$10$hashedpassword" // self-registered account

	gv.On("Verify", mock.Anything, "tok").Return(validPayload(), nil)
	us.On("GetByEmail", mock.Anything, "alice@gmail.com").Return(user, nil)
	us.On("Update", mock.Anything, "user-123", mock.Anything).Return(nil)
	stubDevice(ds)
	ss.On("Put", mock.Anything, mock.AnythingOfType("*domain.Session")).Return(nil)
	jwt.On("Sign", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("bearer", nil)

	result, err := newSvc(us, ss, ds, jwt, gv).LoginWithGoogle(context.Background(), "tok", nil)

	require.NoError(t, err)
	assert.Equal(t, "google-sub-123", result.Session.User.GoogleSub)
	us.AssertCalled(t, "Update", mock.Anything, "user-123", mock.Anything)
}

func TestLoginWithGoogle_NoPasswordAccount_LinkingBlocked(t *testing.T) {
	us, ss, ds, jwt, gv := &mockUserStore{}, &mockSessionStore{}, &mockDeviceStore{}, &mockJWTSigner{}, &mockGoogleVerifier{}

	user := existingUser()
	user.GoogleSub = ""
	user.PasswordHash = "" // admin-provisioned, no password

	gv.On("Verify", mock.Anything, "tok").Return(validPayload(), nil)
	us.On("GetByEmail", mock.Anything, "alice@gmail.com").Return(user, nil)

	_, err := newSvc(us, ss, ds, jwt, gv).LoginWithGoogle(context.Background(), "tok", nil)

	require.Error(t, err)
	assert.True(t, errors.Is(err, domain.ErrUnauthorized))
	us.AssertNotCalled(t, "Update", mock.Anything, mock.Anything, mock.Anything)
}

func TestLoginWithGoogle_SubMismatch_Rejected(t *testing.T) {
	us, ss, ds, jwt, gv := &mockUserStore{}, &mockSessionStore{}, &mockDeviceStore{}, &mockJWTSigner{}, &mockGoogleVerifier{}

	user := existingUser()
	user.GoogleSub = "different-sub"

	gv.On("Verify", mock.Anything, "tok").Return(validPayload(), nil)
	us.On("GetByEmail", mock.Anything, "alice@gmail.com").Return(user, nil)

	_, err := newSvc(us, ss, ds, jwt, gv).LoginWithGoogle(context.Background(), "tok", nil)

	require.Error(t, err)
	assert.True(t, errors.Is(err, domain.ErrUnauthorized))
}

func TestLoginWithGoogle_DisabledAccount(t *testing.T) {
	us, ss, ds, jwt, gv := &mockUserStore{}, &mockSessionStore{}, &mockDeviceStore{}, &mockJWTSigner{}, &mockGoogleVerifier{}

	user := existingUser()
	user.Enable = 0

	gv.On("Verify", mock.Anything, "tok").Return(validPayload(), nil)
	us.On("GetByEmail", mock.Anything, "alice@gmail.com").Return(user, nil)

	_, err := newSvc(us, ss, ds, jwt, gv).LoginWithGoogle(context.Background(), "tok", nil)

	require.Error(t, err)
	assert.True(t, errors.Is(err, domain.ErrUnauthorized))
}

func TestLoginWithGoogle_UnverifiedEmail(t *testing.T) {
	us, ss, ds, jwt, gv := &mockUserStore{}, &mockSessionStore{}, &mockDeviceStore{}, &mockJWTSigner{}, &mockGoogleVerifier{}

	p := validPayload()
	p.EmailVerified = false
	gv.On("Verify", mock.Anything, "tok").Return(p, nil)

	_, err := newSvc(us, ss, ds, jwt, gv).LoginWithGoogle(context.Background(), "tok", nil)

	require.Error(t, err)
	assert.True(t, errors.Is(err, domain.ErrUnauthorized))
}

func TestLoginWithGoogle_EmptyEmail(t *testing.T) {
	us, ss, ds, jwt, gv := &mockUserStore{}, &mockSessionStore{}, &mockDeviceStore{}, &mockJWTSigner{}, &mockGoogleVerifier{}

	p := validPayload()
	p.Email = ""
	gv.On("Verify", mock.Anything, "tok").Return(p, nil)

	_, err := newSvc(us, ss, ds, jwt, gv).LoginWithGoogle(context.Background(), "tok", nil)

	require.Error(t, err)
	assert.True(t, errors.Is(err, domain.ErrUnauthorized))
}

func TestLoginWithGoogle_EmptySub(t *testing.T) {
	us, ss, ds, jwt, gv := &mockUserStore{}, &mockSessionStore{}, &mockDeviceStore{}, &mockJWTSigner{}, &mockGoogleVerifier{}

	p := validPayload()
	p.Sub = ""
	gv.On("Verify", mock.Anything, "tok").Return(p, nil)

	_, err := newSvc(us, ss, ds, jwt, gv).LoginWithGoogle(context.Background(), "tok", nil)

	require.Error(t, err)
	assert.True(t, errors.Is(err, domain.ErrUnauthorized))
}

func TestLoginWithGoogle_VerifierError(t *testing.T) {
	us, ss, ds, jwt, gv := &mockUserStore{}, &mockSessionStore{}, &mockDeviceStore{}, &mockJWTSigner{}, &mockGoogleVerifier{}

	gv.On("Verify", mock.Anything, "bad").Return(nil, domain.ErrUnauthorized)

	_, err := newSvc(us, ss, ds, jwt, gv).LoginWithGoogle(context.Background(), "bad", nil)

	require.Error(t, err)
	assert.True(t, errors.Is(err, domain.ErrUnauthorized))
}

// --- deriveUsername / sanitizeUsername tests ---

func TestSanitizeUsername(t *testing.T) {
	cases := []struct{ input, want string }{
		{"alice.smith", "alice.smith"},
		{"Alice.Smith", "alice.smith"},
		{"alice+tag", "alicetag"},
		{"123user", "123user"},
		{"!@#$%", ""},
		{"alice smith", "alicesmith"},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, sanitizeUsername(c.input), "input: %q", c.input)
	}
}

func TestDeriveUsername_Simple(t *testing.T) {
	us := &mockUserStore{}
	us.On("GetByUsername", mock.Anything, "alice").Return(nil, domain.ErrNotFound)

	svc := &service{userRepo: us}
	username, err := svc.deriveUsername(context.Background(), "alice@gmail.com")

	require.NoError(t, err)
	assert.Equal(t, "alice", username)
}

func TestDeriveUsername_CollisionAddseSuffix(t *testing.T) {
	us := &mockUserStore{}
	us.On("GetByUsername", mock.Anything, "alice").Return(&domain.User{}, nil)   // taken
	us.On("GetByUsername", mock.Anything, "alice1").Return(nil, domain.ErrNotFound) // free

	svc := &service{userRepo: us}
	username, err := svc.deriveUsername(context.Background(), "alice@gmail.com")

	require.NoError(t, err)
	assert.Equal(t, "alice1", username)
}

func TestDeriveUsername_EmptyLocalPartFallsBackToUser(t *testing.T) {
	us := &mockUserStore{}
	us.On("GetByUsername", mock.Anything, "user").Return(nil, domain.ErrNotFound)

	svc := &service{userRepo: us}
	username, err := svc.deriveUsername(context.Background(), "!@gmail.com")

	require.NoError(t, err)
	assert.Equal(t, "user", username)
}

func TestDeriveUsername_ExhaustionReturnsConflict(t *testing.T) {
	us := &mockUserStore{}
	// base + base1..base99 + final check all taken
	us.On("GetByUsername", mock.Anything, mock.Anything).Return(&domain.User{}, nil)

	svc := &service{userRepo: us}
	_, err := svc.deriveUsername(context.Background(), "alice@gmail.com")

	require.Error(t, err)
	assert.True(t, errors.Is(err, domain.ErrConflict))
}
