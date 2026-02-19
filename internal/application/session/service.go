package session

import (
	"context"
	"fmt"
	"time"

	"github.com/go-api-nosql/internal/domain"
	"github.com/go-api-nosql/internal/infrastructure/dynamo"
	jwtinfra "github.com/go-api-nosql/internal/infrastructure/jwt"
	pkgdevice "github.com/go-api-nosql/internal/pkg/device"
	"github.com/go-api-nosql/internal/pkg/id"
	pkgtoken "github.com/go-api-nosql/internal/pkg/token"
	"golang.org/x/crypto/bcrypt"
)



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
	sessionRepo      *dynamo.SessionRepo
	userRepo         *dynamo.UserRepo
	deviceRepo       *dynamo.DeviceRepo
	jwtProvider      *jwtinfra.Provider
	refreshTokenDur  time.Duration
}

func NewService(sessionRepo *dynamo.SessionRepo, userRepo *dynamo.UserRepo, deviceRepo *dynamo.DeviceRepo, jwtProvider *jwtinfra.Provider, refreshTokenDur time.Duration) Service {
	return &service{
		sessionRepo:     sessionRepo,
		userRepo:        userRepo,
		deviceRepo:      deviceRepo,
		jwtProvider:     jwtProvider,
		refreshTokenDur: refreshTokenDur,
	}
}

func (s *service) Login(ctx context.Context, req LoginRequest) (*LoginResult, error) {
	u, err := s.userRepo.GetByUsername(ctx, req.Username)
	if err != nil {
		u, err = s.userRepo.GetByEmail(ctx, req.Username)
		if err != nil {
			return nil, fmt.Errorf("invalid credentials: %w", domain.ErrUnauthorized)
		}
	}
	if !u.Enable {
		return nil, fmt.Errorf("account disabled: %w", domain.ErrUnauthorized)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(req.Password)); err != nil {
		return nil, fmt.Errorf("invalid credentials: %w", domain.ErrUnauthorized)
	}
	dev, err := pkgdevice.Resolve(ctx, s.deviceRepo, req.DeviceUUID, u.UserID)
	if err != nil {
		return nil, err
	}
	refreshToken := pkgtoken.NewRefreshToken()
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
		return nil, fmt.Errorf("session expired: %w", domain.ErrUnauthorized)
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
		return "", "", fmt.Errorf("invalid or expired refresh token: %w", domain.ErrUnauthorized)
	}
	if sess.RefreshExpiresAt < time.Now().Unix() {
		return "", "", fmt.Errorf("refresh token expired: %w", domain.ErrUnauthorized)
	}
	newToken := pkgtoken.NewRefreshToken()
	newExpiry := time.Now().Add(s.refreshTokenDur).Unix()
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

