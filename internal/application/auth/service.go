package auth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"fmt"
	"log/slog"
	"math/big"
	"time"

	"github.com/go-api-nosql/internal/domain"
	"github.com/go-api-nosql/internal/infrastructure/smtp"
	"github.com/go-api-nosql/internal/infrastructure/sns"
	pkgdevice "github.com/go-api-nosql/internal/pkg/device"
	"github.com/go-api-nosql/internal/pkg/id"
	pkgtoken "github.com/go-api-nosql/internal/pkg/token"
	"golang.org/x/crypto/bcrypt"
)

// DynamoDB attribute names used in partial update maps.
const (
	fieldPasswordHash   = "password_hash"
	fieldEmailConfirmed = "email_confirmed"
	fieldPhoneConfirmed = "phone_confirmed"
)

type PasswordRecoveryRequest struct {
	Email       *string `json:"email"`
	PhoneNumber *string `json:"phone_number"`
}

type ValidateOTPRequest struct {
	OTP        string  `json:"otp" validate:"required"`
	DeviceUUID *string `json:"device_uuid"`
	Email      *string `json:"email"`
}

type ChangePasswordRequest struct {
	NewPassword string `json:"new_password" validate:"required"`
}

type ValidateOTPResult struct {
	Bearer       string
	RefreshToken string
	Session      *domain.Session
}

type PasswordRecoveryService interface {
	RequestPasswordRecovery(ctx context.Context, req PasswordRecoveryRequest) error
	ValidateOTP(ctx context.Context, req ValidateOTPRequest) (*ValidateOTPResult, error)
	ChangePassword(ctx context.Context, userID, newPassword string) error
}

type EmailConfirmationService interface {
	RequestEmailConfirmation(ctx context.Context, userID string) error
	ValidateEmailToken(ctx context.Context, userID, token string) error
}

type PhoneConfirmationService interface {
	RequestPhoneConfirmation(ctx context.Context, userID string) error
	ValidatePhoneOTP(ctx context.Context, userID, otp string) error
}

// Service composes the three focused auth sub-services.
type Service interface {
	PasswordRecoveryService
	EmailConfirmationService
	PhoneConfirmationService
}

type verificationStore interface {
	Put(ctx context.Context, v *domain.UserVerification) error
	Get(ctx context.Context, userID, verType string) (*domain.UserVerification, error)
	Delete(ctx context.Context, userID, verType string) error
}

type userStore interface {
	GetByEmail(ctx context.Context, email string) (*domain.User, error)
	Get(ctx context.Context, userID string) (*domain.User, error)
	Update(ctx context.Context, userID string, updates map[string]interface{}) error
}

type sessionStore interface {
	Put(ctx context.Context, s *domain.Session) error
}

type deviceStore interface {
	GetByUUID(ctx context.Context, uuid string) (*domain.Device, error)
	Put(ctx context.Context, d *domain.Device) error
}

type jwtSigner interface {
	Sign(userID, deviceID, role, sessionID string) (string, error)
}

type service struct {
	verificationRepo verificationStore
	userRepo         userStore
	sessionRepo      sessionStore
	deviceRepo       deviceStore
	mailer           smtp.Mailer
	smsSender        sns.SMSSender
	jwtProvider      jwtSigner
	refreshTokenDur  time.Duration
}

type ServiceDeps struct {
	VerificationRepo verificationStore
	UserRepo         userStore
	SessionRepo      sessionStore
	DeviceRepo       deviceStore
	Mailer           smtp.Mailer
	SMSSender        sns.SMSSender
	JWTProvider      jwtSigner
	RefreshTokenDur  time.Duration
}

func NewService(deps ServiceDeps) Service {
	return &service{
		verificationRepo: deps.VerificationRepo,
		userRepo:         deps.UserRepo,
		sessionRepo:      deps.SessionRepo,
		deviceRepo:       deps.DeviceRepo,
		mailer:           deps.Mailer,
		smsSender:        deps.SMSSender,
		jwtProvider:      deps.JWTProvider,
		refreshTokenDur:  deps.RefreshTokenDur,
	}
}

func (s *service) RequestPasswordRecovery(ctx context.Context, req PasswordRecoveryRequest) error {
	var u *domain.User
	var err error
	switch {
	case req.Email != nil:
		u, err = s.userRepo.GetByEmail(ctx, *req.Email)
		if err != nil {
			return fmt.Errorf("user not found: %w", domain.ErrNotFound)
		}
	case req.PhoneNumber != nil:
		return fmt.Errorf("phone recovery not supported; provide email: %w", domain.ErrBadRequest)
	default:
		return fmt.Errorf("email or phone_number required: %w", domain.ErrBadRequest)
	}

	otp, err := generateOTP()
	if err != nil {
		return err
	}

	v := &domain.UserVerification{
		UserID:    u.UserID,
		Type:      "otp",
		Code:      otp,
		ExpiresAt: time.Now().Add(15 * time.Minute).Unix(),
	}
	if err := s.verificationRepo.Put(ctx, v); err != nil {
		return err
	}

	return s.mailer.SendEmail(u.Email, "Password Recovery OTP", "Your OTP: "+otp)
}

func (s *service) ValidateOTP(ctx context.Context, req ValidateOTPRequest) (*ValidateOTPResult, error) {
	if req.Email == nil {
		return nil, fmt.Errorf("email required to validate OTP: %w", domain.ErrBadRequest)
	}
	u, err := s.userRepo.GetByEmail(ctx, *req.Email)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", domain.ErrNotFound)
	}
	v, err := s.verificationRepo.Get(ctx, u.UserID, "otp")
	if err != nil {
		return nil, fmt.Errorf("OTP not found: %w", domain.ErrNotFound)
	}
	if subtle.ConstantTimeCompare([]byte(v.Code), []byte(req.OTP)) != 1 {
		return nil, fmt.Errorf("invalid OTP: %w", domain.ErrUnauthorized)
	}
	if v.ExpiresAt < time.Now().Unix() {
		return nil, fmt.Errorf("OTP expired: %w", domain.ErrUnauthorized)
	}
	if err := s.verificationRepo.Delete(ctx, u.UserID, "otp"); err != nil {
		slog.Warn("failed to delete OTP verification record", "user_id", u.UserID, "err", err)
	}

	dev, err := pkgdevice.Resolve(ctx, s.deviceRepo, req.DeviceUUID, u.UserID)
	if err != nil {
		return nil, err
	}
	refreshToken, err := pkgtoken.NewRefreshToken()
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	sess := &domain.Session{
		SessionID:        id.New(),
		UserID:           u.UserID,
		DeviceID:         dev.DeviceID,
		Enable:           true,
		RefreshToken:     refreshToken,
		RefreshExpiresAt: now.Add(s.refreshTokenDur).Unix(),
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := s.sessionRepo.Put(ctx, sess); err != nil {
		return nil, err
	}
	bearer, err := s.jwtProvider.Sign(u.UserID, dev.DeviceID, u.Role, sess.SessionID)
	if err != nil {
		return nil, err
	}
	sess.User = u
	return &ValidateOTPResult{Bearer: bearer, RefreshToken: refreshToken, Session: sess}, nil
}

func (s *service) ChangePassword(ctx context.Context, userID, newPassword string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return s.userRepo.Update(ctx, userID, map[string]interface{}{fieldPasswordHash: string(hash)})
}

func (s *service) RequestEmailConfirmation(ctx context.Context, userID string) error {
	token, err := generateToken(32)
	if err != nil {
		return err
	}
	v := &domain.UserVerification{
		UserID:    userID,
		Type:      "email",
		Code:      token,
		ExpiresAt: time.Now().Add(24 * time.Hour).Unix(),
	}
	if err := s.verificationRepo.Put(ctx, v); err != nil {
		return err
	}
	u, err := s.userRepo.Get(ctx, userID)
	if err != nil {
		return err
	}
	return s.mailer.SendEmail(u.Email, "Confirm your email", "Token: "+token)
}

func (s *service) ValidateEmailToken(ctx context.Context, userID, token string) error {
	v, err := s.verificationRepo.Get(ctx, userID, "email")
	if err != nil {
		return fmt.Errorf("token not found: %w", domain.ErrNotFound)
	}
	if subtle.ConstantTimeCompare([]byte(v.Code), []byte(token)) != 1 {
		return fmt.Errorf("invalid token: %w", domain.ErrUnauthorized)
	}
	if v.ExpiresAt < time.Now().Unix() {
		return fmt.Errorf("token expired: %w", domain.ErrUnauthorized)
	}
	if err := s.verificationRepo.Delete(ctx, userID, "email"); err != nil {
		slog.Warn("failed to delete email verification record", "user_id", userID, "err", err)
	}
	return s.userRepo.Update(ctx, userID, map[string]interface{}{fieldEmailConfirmed: true})
}

func (s *service) RequestPhoneConfirmation(ctx context.Context, userID string) error {
	u, err := s.userRepo.Get(ctx, userID)
	if err != nil {
		return fmt.Errorf("user not found: %w", domain.ErrNotFound)
	}
	if u.Phone == nil {
		return fmt.Errorf("no phone number on account: %w", domain.ErrBadRequest)
	}
	otp, err := generateOTP()
	if err != nil {
		return err
	}
	v := &domain.UserVerification{
		UserID:    userID,
		Type:      "phone",
		Code:      otp,
		ExpiresAt: time.Now().Add(15 * time.Minute).Unix(),
	}
	if err := s.verificationRepo.Put(ctx, v); err != nil {
		return err
	}
	return s.smsSender.SendSMS(ctx, *u.Phone, "Your verification code: "+otp)
}

func (s *service) ValidatePhoneOTP(ctx context.Context, userID, otp string) error {
	v, err := s.verificationRepo.Get(ctx, userID, "phone")
	if err != nil {
		return fmt.Errorf("OTP not found: %w", domain.ErrNotFound)
	}
	if subtle.ConstantTimeCompare([]byte(v.Code), []byte(otp)) != 1 {
		return fmt.Errorf("invalid OTP: %w", domain.ErrUnauthorized)
	}
	if v.ExpiresAt < time.Now().Unix() {
		return fmt.Errorf("OTP expired: %w", domain.ErrUnauthorized)
	}
	if err := s.verificationRepo.Delete(ctx, userID, "phone"); err != nil {
		slog.Warn("failed to delete phone verification record", "user_id", userID, "err", err)
	}
	return s.userRepo.Update(ctx, userID, map[string]interface{}{fieldPhoneConfirmed: true})
}

// generateOTP returns a 6-character cryptographically random uppercase alphanumeric code,
// excluding visually ambiguous characters (0, 1, I, L, O) for easier manual entry.
func generateOTP() (string, error) {
	const chars = "23456789ABCDEFGHJKMNPQRSTUVWXYZ"
	b := make([]byte, 6)
	for i := range b {
		idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		if err != nil {
			return "", err
		}
		b[i] = chars[idx.Int64()]
	}
	return string(b), nil
}

func generateToken(n int) (string, error) {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, n)
	for i := range b {
		idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(letters))))
		if err != nil {
			return "", err
		}
		b[i] = letters[idx.Int64()]
	}
	return string(b), nil
}
