package session

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"github.com/go-api-nosql/internal/domain"
	"github.com/go-api-nosql/internal/infrastructure/dynamo"
	jwtinfra "github.com/go-api-nosql/internal/infrastructure/jwt"
	"github.com/go-api-nosql/internal/pkg/id"
	"golang.org/x/crypto/bcrypt"
)

const refreshTokenDuration = 30 * 24 * time.Hour

type LoginRequest struct {
	Username   string  `json:"username" validate:"required"`
	Password   string  `json:"password" validate:"required"`
	DeviceUUID *string `json:"device_uuid"`
}

type LoginResult struct {
	Bearer       string
	RefreshToken string
	Session      *domain.Session
}

type Service interface {
	Login(ctx context.Context, req LoginRequest) (*LoginResult, error)
	Logout(ctx context.Context, sessionID string) error
	GetCurrent(ctx context.Context, sessionID string) (*domain.Session, error)
	Refresh(ctx context.Context, refreshToken string) (bearer, newRefreshToken string, err error)
}

type service struct {
	sessionRepo *dynamo.SessionRepo
	userRepo    *dynamo.UserRepo
	deviceRepo  *dynamo.DeviceRepo
	jwtProvider *jwtinfra.Provider
}

func NewService(sessionRepo *dynamo.SessionRepo, userRepo *dynamo.UserRepo, deviceRepo *dynamo.DeviceRepo, jwtProvider *jwtinfra.Provider) Service {
	return &service{
		sessionRepo: sessionRepo,
		userRepo:    userRepo,
		deviceRepo:  deviceRepo,
		jwtProvider: jwtProvider,
	}
}

func (s *service) Login(ctx context.Context, req LoginRequest) (*LoginResult, error) {
	u, err := s.userRepo.GetByUsername(ctx, req.Username)
	if err != nil {
		u, err = s.userRepo.GetByEmail(ctx, req.Username)
		if err != nil {
			return nil, errors.New("invalid credentials")
		}
	}
	if !u.Enable {
		return nil, errors.New("account disabled")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(req.Password)); err != nil {
		return nil, errors.New("invalid credentials")
	}
	device, err := s.resolveDevice(ctx, req.DeviceUUID, u.UserID)
	if err != nil {
		return nil, err
	}
	refreshToken := newRefreshToken()
	now := time.Now().UTC()
	sess := &domain.Session{
		SessionID:        id.New(),
		UserID:           u.UserID,
		DeviceID:         device.DeviceID,
		Enable:           true,
		RefreshToken:     refreshToken,
		RefreshExpiresAt: now.Add(refreshTokenDuration).Unix(),
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := s.sessionRepo.Put(ctx, sess); err != nil {
		return nil, err
	}
	bearer, err := s.jwtProvider.Sign(u.UserID, device.DeviceID, u.Role, sess.SessionID)
	if err != nil {
		return nil, err
	}
	sess.User = u
	return &LoginResult{Bearer: bearer, RefreshToken: refreshToken, Session: sess}, nil
}

func (s *service) Logout(ctx context.Context, sessionID string) error {
	return s.sessionRepo.Update(ctx, sessionID, map[string]interface{}{"enable": false})
}

func (s *service) GetCurrent(ctx context.Context, sessionID string) (*domain.Session, error) {
	sess, err := s.sessionRepo.Get(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if !sess.Enable {
		return nil, errors.New("session expired")
	}
	u, err := s.userRepo.Get(ctx, sess.UserID)
	if err != nil {
		return nil, err
	}
	sess.User = u
	return sess, nil
}

func (s *service) Refresh(ctx context.Context, refreshToken string) (string, string, error) {
	sess, err := s.sessionRepo.GetByRefreshToken(ctx, refreshToken)
	if err != nil {
		return "", "", errors.New("invalid or expired refresh token")
	}
	if sess.RefreshExpiresAt < time.Now().Unix() {
		return "", "", errors.New("refresh token expired")
	}
	newToken := newRefreshToken()
	newExpiry := time.Now().Add(refreshTokenDuration).Unix()
	if err := s.sessionRepo.RotateRefreshToken(ctx, sess.SessionID, newToken, newExpiry); err != nil {
		return "", "", err
	}
	u, err := s.userRepo.Get(ctx, sess.UserID)
	if err != nil {
		return "", "", err
	}
	bearer, err := s.jwtProvider.Sign(u.UserID, sess.DeviceID, u.Role, sess.SessionID)
	if err != nil {
		return "", "", err
	}
	return bearer, newToken, nil
}

func (s *service) resolveDevice(ctx context.Context, deviceUUID *string, userID string) (*domain.Device, error) {
	if deviceUUID != nil {
		if d, err := s.deviceRepo.GetByUUID(ctx, *deviceUUID); err == nil {
			return d, nil
		}
	}
	devUUID := id.New()
	if deviceUUID != nil {
		devUUID = *deviceUUID
	}
	now := time.Now().UTC()
	d := &domain.Device{
		DeviceID:  id.New(),
		UUID:      devUUID,
		UserID:    userID,
		Enable:    true,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.deviceRepo.Put(ctx, d); err != nil {
		return nil, err
	}
	return d, nil
}

func newRefreshToken() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
