package auth

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

type mockVerificationStore struct{ mock.Mock }

func (m *mockVerificationStore) Put(ctx context.Context, v *domain.UserVerification) error {
	return m.Called(ctx, v).Error(0)
}
func (m *mockVerificationStore) Get(ctx context.Context, userID, verType string) (*domain.UserVerification, error) {
	args := m.Called(ctx, userID, verType)
	if v, _ := args.Get(0).(*domain.UserVerification); v != nil {
		return v, args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *mockVerificationStore) Delete(ctx context.Context, userID, verType string) error {
	return m.Called(ctx, userID, verType).Error(0)
}

type mockUserStore struct{ mock.Mock }

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
func (m *mockUserStore) Update(ctx context.Context, userID string, updates map[string]interface{}) error {
	return m.Called(ctx, userID, updates).Error(0)
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

type mockMailer struct{ mock.Mock }

func (m *mockMailer) SendEmail(to, subject, body string) error {
	return m.Called(to, subject, body).Error(0)
}

type mockSMSSender struct{ mock.Mock }

func (m *mockSMSSender) SendSMS(ctx context.Context, phone, msg string) error {
	return m.Called(ctx, phone, msg).Error(0)
}

type mockJWTSigner struct{ mock.Mock }

func (m *mockJWTSigner) Sign(userID, deviceID, role, sessionID string) (string, error) {
	args := m.Called(userID, deviceID, role, sessionID)
	return args.String(0), args.Error(1)
}

// --- builder ---

func newService(vs *mockVerificationStore, us *mockUserStore, ss *mockSessionStore, ds *mockDeviceStore, ml *mockMailer, sms *mockSMSSender, jwt *mockJWTSigner) Service {
	return NewService(ServiceDeps{
		VerificationRepo: vs,
		UserRepo:         us,
		SessionRepo:      ss,
		DeviceRepo:       ds,
		Mailer:           ml,
		SMSSender:        sms,
		JWTProvider:      jwt,
		RefreshTokenDur:  7 * 24 * time.Hour,
	})
}

// --- RequestPasswordRecovery ---

func TestRequestPasswordRecovery_EmailNotFound(t *testing.T) {
	us := &mockUserStore{}
	us.On("GetByEmail", mock.Anything, "x@x.com").Return(nil, domain.ErrNotFound)

	svc := newService(nil, us, nil, nil, nil, nil, nil)
	err := svc.RequestPasswordRecovery(context.Background(), PasswordRecoveryRequest{
		Email: strPtr("x@x.com"),
	})

	require.Error(t, err)
	assert.True(t, errors.Is(err, domain.ErrNotFound))
}

func TestRequestPasswordRecovery_PhoneBranch_ReturnsBadRequest(t *testing.T) {
	svc := newService(nil, nil, nil, nil, nil, nil, nil)
	phone := "5551234"
	err := svc.RequestPasswordRecovery(context.Background(), PasswordRecoveryRequest{
		PhoneNumber: &phone,
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, domain.ErrBadRequest))
}

func TestRequestPasswordRecovery_NoField_ReturnsBadRequest(t *testing.T) {
	svc := newService(nil, nil, nil, nil, nil, nil, nil)
	err := svc.RequestPasswordRecovery(context.Background(), PasswordRecoveryRequest{})
	require.Error(t, err)
	assert.True(t, errors.Is(err, domain.ErrBadRequest))
}

func TestRequestPasswordRecovery_HappyPath(t *testing.T) {
	us := &mockUserStore{}
	vs := &mockVerificationStore{}
	ml := &mockMailer{}

	user := &domain.User{UserID: "u1", Email: "a@b.com"}
	us.On("GetByEmail", mock.Anything, "a@b.com").Return(user, nil)
	vs.On("Get", mock.Anything, "u1", "otp").Return(nil, domain.ErrNotFound) // no existing OTP — cooldown check passes
	vs.On("Put", mock.Anything, mock.AnythingOfType("*domain.UserVerification")).Return(nil)
	ml.On("SendEmail", "a@b.com", mock.Anything, mock.Anything).Return(nil)

	svc := newService(vs, us, nil, nil, ml, nil, nil)
	err := svc.RequestPasswordRecovery(context.Background(), PasswordRecoveryRequest{
		Email: strPtr("a@b.com"),
	})

	require.NoError(t, err)
	us.AssertExpectations(t)
	vs.AssertExpectations(t)
	ml.AssertExpectations(t)
}

// --- ValidateOTP ---

func TestValidateOTP_NoEmail_ReturnsBadRequest(t *testing.T) {
	svc := newService(nil, nil, nil, nil, nil, nil, nil)
	_, err := svc.ValidateOTP(context.Background(), ValidateOTPRequest{OTP: "123456"})
	require.Error(t, err)
	assert.True(t, errors.Is(err, domain.ErrBadRequest))
}

func TestValidateOTP_UserNotFound(t *testing.T) {
	us := &mockUserStore{}
	us.On("GetByEmail", mock.Anything, "a@b.com").Return(nil, domain.ErrNotFound)

	svc := newService(nil, us, nil, nil, nil, nil, nil)
	_, err := svc.ValidateOTP(context.Background(), ValidateOTPRequest{
		OTP:   "123456",
		Email: strPtr("a@b.com"),
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, domain.ErrNotFound))
}

func TestValidateOTP_OTPNotFound(t *testing.T) {
	us := &mockUserStore{}
	vs := &mockVerificationStore{}
	user := &domain.User{UserID: "u1"}
	us.On("GetByEmail", mock.Anything, "a@b.com").Return(user, nil)
	vs.On("Get", mock.Anything, "u1", "otp").Return(nil, domain.ErrNotFound)

	svc := newService(vs, us, nil, nil, nil, nil, nil)
	_, err := svc.ValidateOTP(context.Background(), ValidateOTPRequest{
		OTP:   "123456",
		Email: strPtr("a@b.com"),
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, domain.ErrNotFound))
}

func TestValidateOTP_InvalidOTP(t *testing.T) {
	us := &mockUserStore{}
	vs := &mockVerificationStore{}
	user := &domain.User{UserID: "u1"}
	us.On("GetByEmail", mock.Anything, "a@b.com").Return(user, nil)
	vs.On("Get", mock.Anything, "u1", "otp").Return(&domain.UserVerification{
		Code:      "AAAAAA",
		ExpiresAt: time.Now().Add(10 * time.Minute).Unix(),
	}, nil)

	svc := newService(vs, us, nil, nil, nil, nil, nil)
	_, err := svc.ValidateOTP(context.Background(), ValidateOTPRequest{
		OTP:         "BBBBBB",
		NewPassword: "newpassword123",
		Email:       strPtr("a@b.com"),
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, domain.ErrUnauthorized))
}

func TestValidateOTP_ExpiredOTP(t *testing.T) {
	us := &mockUserStore{}
	vs := &mockVerificationStore{}
	user := &domain.User{UserID: "u1"}
	us.On("GetByEmail", mock.Anything, "a@b.com").Return(user, nil)
	vs.On("Get", mock.Anything, "u1", "otp").Return(&domain.UserVerification{
		Code:      "AAAAAA",
		ExpiresAt: time.Now().Add(-1 * time.Minute).Unix(), // expired
	}, nil)

	svc := newService(vs, us, nil, nil, nil, nil, nil)
	_, err := svc.ValidateOTP(context.Background(), ValidateOTPRequest{
		OTP:         "AAAAAA",
		NewPassword: "newpassword123",
		Email:       strPtr("a@b.com"),
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, domain.ErrUnauthorized))
}

func TestValidateOTP_HappyPath(t *testing.T) {
	us := &mockUserStore{}
	vs := &mockVerificationStore{}
	ss := &mockSessionStore{}
	ds := &mockDeviceStore{}
	jwt := &mockJWTSigner{}

	user := &domain.User{UserID: "u1", Email: "a@b.com", Role: domain.RoleUser}
	us.On("GetByEmail", mock.Anything, "a@b.com").Return(user, nil)
	vs.On("Get", mock.Anything, "u1", "otp").Return(&domain.UserVerification{
		Code:      "AAAAAA",
		ExpiresAt: time.Now().Add(10 * time.Minute).Unix(),
	}, nil)
	vs.On("Delete", mock.Anything, "u1", "otp").Return(nil)
	us.On("Update", mock.Anything, "u1", mock.MatchedBy(func(m map[string]interface{}) bool {
		_, ok := m[fieldPasswordHash]
		return ok
	})).Return(nil)
	ds.On("GetByUUID", mock.Anything, mock.Anything).Return(nil, domain.ErrNotFound)
	ds.On("Put", mock.Anything, mock.AnythingOfType("*domain.Device")).Return(nil)
	ss.On("SoftDeleteByUser", mock.Anything, "u1").Return(nil)
	ss.On("Put", mock.Anything, mock.AnythingOfType("*domain.Session")).Return(nil)
	jwt.On("Sign", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("bearer-token", nil)

	svc := newService(vs, us, ss, ds, nil, nil, jwt)
	result, err := svc.ValidateOTP(context.Background(), ValidateOTPRequest{
		OTP:         "AAAAAA",
		NewPassword: "newpassword123",
		Email:       strPtr("a@b.com"),
	})

	require.NoError(t, err)
	assert.Equal(t, "bearer-token", result.Bearer)
	assert.NotEmpty(t, result.RefreshToken)
}

func strPtr(s string) *string { return &s }
