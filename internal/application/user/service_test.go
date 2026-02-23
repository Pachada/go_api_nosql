package user

import (
	"context"
	"errors"
	"testing"

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
func (m *mockUserStore) Put(ctx context.Context, u *domain.User) error {
	return m.Called(ctx, u).Error(0)
}
func (m *mockUserStore) QueryPage(ctx context.Context, limit int32, cursor string) ([]domain.User, string, error) {
	args := m.Called(ctx, limit, cursor)
	return args.Get(0).([]domain.User), args.String(1), args.Error(2)
}
func (m *mockUserStore) Get(ctx context.Context, userID string) (*domain.User, error) {
	args := m.Called(ctx, userID)
	if u, _ := args.Get(0).(*domain.User); u != nil {
		return u, args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *mockUserStore) Update(ctx context.Context, userID string, updates map[string]interface{}) error {
	return m.Called(ctx, userID, updates).Error(0)
}
func (m *mockUserStore) SoftDelete(ctx context.Context, userID string) error {
	return m.Called(ctx, userID).Error(0)
}

type mockSessionStore struct{ mock.Mock }

func (m *mockSessionStore) Put(ctx context.Context, s *domain.Session) error {
	return m.Called(ctx, s).Error(0)
}
func (m *mockSessionStore) SoftDeleteByUser(ctx context.Context, userID string) error {
	return m.Called(ctx, userID).Error(0)
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

// --- helpers ---

func newService(us *mockUserStore, ss *mockSessionStore, ds *mockDeviceStore, jwt *mockJWTSigner) Service {
	return NewService(ServiceDeps{
		UserRepo:    us,
		SessionRepo: ss,
		DeviceRepo:  ds,
		JWTProvider: jwt,
	})
}

func baseReq() domain.CreateUserRequest {
	return domain.CreateUserRequest{
		Username:  "alice",
		Password:  "password123",
		Email:     "alice@example.com",
		FirstName: "Alice",
		LastName:  "Smith",
	}
}

// --- Register tests ---

func TestRegister_UsernameConflict(t *testing.T) {
	us := &mockUserStore{}
	us.On("GetByUsername", mock.Anything, "alice").Return(&domain.User{}, nil)

	svc := newService(us, nil, nil, nil)
	_, err := svc.Register(context.Background(), baseReq())

	require.Error(t, err)
	assert.True(t, errors.Is(err, domain.ErrConflict))
	us.AssertExpectations(t)
}

func TestRegister_EmailConflict(t *testing.T) {
	us := &mockUserStore{}
	us.On("GetByUsername", mock.Anything, "alice").Return(nil, domain.ErrNotFound)
	us.On("GetByEmail", mock.Anything, "alice@example.com").Return(&domain.User{}, nil)

	svc := newService(us, nil, nil, nil)
	_, err := svc.Register(context.Background(), baseReq())

	require.Error(t, err)
	assert.True(t, errors.Is(err, domain.ErrConflict))
	us.AssertExpectations(t)
}

func TestRegister_InvalidBirthday(t *testing.T) {
	us := &mockUserStore{}
	us.On("GetByUsername", mock.Anything, "alice").Return(nil, domain.ErrNotFound)
	us.On("GetByEmail", mock.Anything, "alice@example.com").Return(nil, domain.ErrNotFound)

	svc := newService(us, nil, nil, nil)
	req := baseReq()
	req.Birthday = "not-a-date"
	_, err := svc.Register(context.Background(), req)

	require.Error(t, err)
	assert.True(t, errors.Is(err, domain.ErrBadRequest))
}

func TestRegister_HappyPath(t *testing.T) {
	us := &mockUserStore{}
	us.On("GetByUsername", mock.Anything, "alice").Return(nil, domain.ErrNotFound)
	us.On("GetByEmail", mock.Anything, "alice@example.com").Return(nil, domain.ErrNotFound)
	us.On("Put", mock.Anything, mock.AnythingOfType("*domain.User")).Return(nil)

	svc := newService(us, nil, nil, nil)
	u, err := svc.Register(context.Background(), baseReq())

	require.NoError(t, err)
	assert.Equal(t, "alice", u.Username)
	assert.Equal(t, domain.RoleUser, u.Role)
	assert.Equal(t, 1, u.Enable)
	us.AssertExpectations(t)
}

// --- Update tests ---

func ptr[T any](v T) *T { return &v }

func TestUpdate_EmptyRequest_ReturnsExistingUser(t *testing.T) {
	us := &mockUserStore{}
	existing := &domain.User{UserID: "u1", Username: "alice"}
	us.On("Get", mock.Anything, "u1").Return(existing, nil)

	svc := newService(us, nil, nil, nil)
	u, err := svc.Update(context.Background(), "u1", domain.UpdateUserRequest{})

	require.NoError(t, err)
	assert.Equal(t, existing, u)
	us.AssertExpectations(t)
}

func TestUpdate_InvalidBirthday(t *testing.T) {
	svc := newService(&mockUserStore{}, nil, nil, nil)
	_, err := svc.Update(context.Background(), "u1", domain.UpdateUserRequest{
		Birthday: ptr("bad-date"),
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, domain.ErrBadRequest))
}

func TestUpdate_InvalidRole(t *testing.T) {
	svc := newService(&mockUserStore{}, nil, nil, nil)
	_, err := svc.Update(context.Background(), "u1", domain.UpdateUserRequest{
		Role: ptr("superuser"),
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, domain.ErrBadRequest))
}

func TestUpdate_HappyPath(t *testing.T) {
	us := &mockUserStore{}
	updated := &domain.User{UserID: "u1", Username: "bob"}
	us.On("Update", mock.Anything, "u1", mock.Anything).Return(nil)
	us.On("Get", mock.Anything, "u1").Return(updated, nil)

	svc := newService(us, nil, nil, nil)
	u, err := svc.Update(context.Background(), "u1", domain.UpdateUserRequest{
		Username: ptr("bob"),
	})

	require.NoError(t, err)
	assert.Equal(t, "bob", u.Username)
	us.AssertExpectations(t)
}

// --- Delete tests ---

func TestDelete_PropagatesStoreError(t *testing.T) {
	us := &mockUserStore{}
	storeErr := errors.New("dynamo error")
	us.On("SoftDelete", mock.Anything, "u1").Return(storeErr)

	svc := newService(us, &mockSessionStore{}, nil, nil)
	err := svc.Delete(context.Background(), "u1")

	require.Error(t, err)
	assert.Equal(t, storeErr, err)
	us.AssertExpectations(t)
}

func TestDelete_AlsoDeletesSessions(t *testing.T) {
	us := &mockUserStore{}
	ss := &mockSessionStore{}
	us.On("SoftDelete", mock.Anything, "u1").Return(nil)
	ss.On("SoftDeleteByUser", mock.Anything, "u1").Return(nil)

	svc := newService(us, ss, nil, nil)
	err := svc.Delete(context.Background(), "u1")

	require.NoError(t, err)
	us.AssertExpectations(t)
	ss.AssertExpectations(t)
}
