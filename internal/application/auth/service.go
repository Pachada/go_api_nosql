package auth

import (
	"context"
	"crypto/rand"
	"fmt"
	"log/slog"
	"math/big"
	"time"

	"github.com/go-api-nosql/internal/domain"
	"github.com/go-api-nosql/internal/infrastructure/dynamo"
	jwtinfra "github.com/go-api-nosql/internal/infrastructure/jwt"
	"github.com/go-api-nosql/internal/infrastructure/smtp"
	"github.com/go-api-nosql/internal/infrastructure/sns"
	pkgdevice "github.com/go-api-nosql/internal/pkg/device"
	"github.com/go-api-nosql/internal/pkg/id"
	pkgtoken "github.com/go-api-nosql/internal/pkg/token"
	"golang.org/x/crypto/bcrypt"
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

type Service interface {
	RequestPasswordRecovery(ctx context.Context, req PasswordRecoveryRequest) error
	ValidateOTP(ctx context.Context, req ValidateOTPRequest) (bearer, refreshToken string, session *domain.Session, err error)
	ChangePassword(ctx context.Context, userID, newPassword string) error
	RequestEmailConfirmation(ctx context.Context, userID string) error
	ValidateEmailToken(ctx context.Context, userID, token string) error
	RequestPhoneConfirmation(ctx context.Context, userID string) error
	ValidatePhoneOTP(ctx context.Context, userID, otp string) error
}

type service struct {
	verificationRepo *dynamo.VerificationRepo
	userRepo         *dynamo.UserRepo
	sessionRepo      *dynamo.SessionRepo
	deviceRepo       *dynamo.DeviceRepo
	mailer           smtp.Mailer
	smsSender        sns.SMSSender
	jwtProvider      *jwtinfra.Provider
	refreshTokenDur  time.Duration
}

func NewService(
	verificationRepo *dynamo.VerificationRepo,
	userRepo *dynamo.UserRepo,
	sessionRepo *dynamo.SessionRepo,
	deviceRepo *dynamo.DeviceRepo,
	mailer smtp.Mailer,
	smsSender sns.SMSSender,
	jwtProvider *jwtinfra.Provider,
	refreshTokenDur time.Duration,
) Service {
	return &service{
		verificationRepo: verificationRepo,
		userRepo:         userRepo,
		sessionRepo:      sessionRepo,
		deviceRepo:       deviceRepo,
		mailer:           mailer,
		smsSender:        smsSender,
		jwtProvider:      jwtProvider,
		refreshTokenDur:  refreshTokenDur,
	}
}

func (s *service) RequestPasswordRecovery(ctx context.Context, req PasswordRecoveryRequest) error {
	var u *domain.User
	var err error
	if req.Email != nil {
		u, err = s.userRepo.GetByEmail(ctx, *req.Email)
		if err != nil {
			return fmt.Errorf("user not found: %w", domain.ErrNotFound)
		}
	} else if req.PhoneNumber != nil {
		return fmt.Errorf("phone recovery not supported; provide email: %w", domain.ErrBadRequest)
	} else {
		return fmt.Errorf("email or phone_number required: %w", domain.ErrBadRequest)
	}

	n, err := rand.Int(rand.Reader, big.NewInt(999999))
	if err != nil {
		return err
	}
	otp := fmt.Sprintf("%06d", n.Int64())

	v := &domain.UserVerification{
		UserID:    u.UserID,
		Type:      "otp",
		Code:      otp,
		ExpiresAt: time.Now().Add(15 * time.Minute).Unix(),
	}
	if err := s.verificationRepo.Put(ctx, v); err != nil {
		return err
	}

	if req.Email != nil {
		return s.mailer.SendEmail(u.Email, "Password Recovery OTP", "Your OTP: "+otp)
	}
	return s.smsSender.SendSMS(*req.PhoneNumber, "Your OTP: "+otp)
}

func (s *service) ValidateOTP(ctx context.Context, req ValidateOTPRequest) (string, string, *domain.Session, error) {
	if req.Email == nil {
		return "", "", nil, fmt.Errorf("email required to validate OTP: %w", domain.ErrBadRequest)
	}
	u, err := s.userRepo.GetByEmail(ctx, *req.Email)
	if err != nil {
		return "", "", nil, fmt.Errorf("user not found: %w", domain.ErrNotFound)
	}
	v, err := s.verificationRepo.Get(ctx, u.UserID, "otp")
	if err != nil {
		return "", "", nil, fmt.Errorf("OTP not found: %w", domain.ErrNotFound)
	}
	if v.Code != req.OTP {
		return "", "", nil, fmt.Errorf("invalid OTP: %w", domain.ErrUnauthorized)
	}
	if v.ExpiresAt < time.Now().Unix() {
		return "", "", nil, fmt.Errorf("OTP expired: %w", domain.ErrUnauthorized)
	}
	if err := s.verificationRepo.Delete(ctx, u.UserID, "otp"); err != nil {
		slog.Warn("failed to delete OTP verification record", "user_id", u.UserID, "err", err)
	}

	dev, err := pkgdevice.Resolve(ctx, s.deviceRepo, req.DeviceUUID, u.UserID)
	if err != nil {
		return "", "", nil, err
	}
	refreshToken, err := pkgtoken.NewRefreshToken()
	if err != nil {
		return "", "", nil, err
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
		return "", "", nil, err
	}
	bearer, err := s.jwtProvider.Sign(u.UserID, dev.DeviceID, u.Role, sess.SessionID)
	if err != nil {
		return "", "", nil, err
	}
	sess.User = u
	return bearer, refreshToken, sess, nil
}

func (s *service) ChangePassword(ctx context.Context, userID, newPassword string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return s.userRepo.Update(ctx, userID, map[string]interface{}{"password_hash": string(hash)})
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
	if v.Code != token {
		return fmt.Errorf("invalid token: %w", domain.ErrUnauthorized)
	}
	if v.ExpiresAt < time.Now().Unix() {
		return fmt.Errorf("token expired: %w", domain.ErrUnauthorized)
	}
	if err := s.verificationRepo.Delete(ctx, userID, "email"); err != nil {
		slog.Warn("failed to delete email verification record", "user_id", userID, "err", err)
	}
	return s.userRepo.Update(ctx, userID, map[string]interface{}{"email_confirmed": true})
}

func (s *service) RequestPhoneConfirmation(ctx context.Context, userID string) error {
	u, err := s.userRepo.Get(ctx, userID)
	if err != nil {
		return fmt.Errorf("user not found: %w", domain.ErrNotFound)
	}
	if u.Phone == nil {
		return fmt.Errorf("no phone number on account: %w", domain.ErrBadRequest)
	}
	n, err := rand.Int(rand.Reader, big.NewInt(999999))
	if err != nil {
		return err
	}
	otp := fmt.Sprintf("%06d", n.Int64())
	v := &domain.UserVerification{
		UserID:    userID,
		Type:      "phone",
		Code:      otp,
		ExpiresAt: time.Now().Add(15 * time.Minute).Unix(),
	}
	if err := s.verificationRepo.Put(ctx, v); err != nil {
		return err
	}
	return s.smsSender.SendSMS(*u.Phone, "Your verification code: "+otp)
}

func (s *service) ValidatePhoneOTP(ctx context.Context, userID, otp string) error {
	v, err := s.verificationRepo.Get(ctx, userID, "phone")
	if err != nil {
		return fmt.Errorf("OTP not found: %w", domain.ErrNotFound)
	}
	if v.Code != otp {
		return fmt.Errorf("invalid OTP: %w", domain.ErrUnauthorized)
	}
	if v.ExpiresAt < time.Now().Unix() {
		return fmt.Errorf("OTP expired: %w", domain.ErrUnauthorized)
	}
	if err := s.verificationRepo.Delete(ctx, userID, "phone"); err != nil {
		slog.Warn("failed to delete phone verification record", "user_id", userID, "err", err)
	}
	return s.userRepo.Update(ctx, userID, map[string]interface{}{"phone_confirmed": true})
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


