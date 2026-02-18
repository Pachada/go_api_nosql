package session

import (
	"context"
	"errors"
	"time"

	"github.com/go-api-nosql/internal/domain"
	"github.com/go-api-nosql/internal/infrastructure/dynamo"
	jwtinfra "github.com/go-api-nosql/internal/infrastructure/jwt"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type LoginRequest struct {
	Username   string  `json:"username" validate:"required"`
	Password   string  `json:"password" validate:"required"`
	DeviceUUID *string `json:"device_uuid"`
}

type LoginResult struct {
	Bearer  string
	Session *domain.Session
}

type Service interface {
	Login(ctx context.Context, req LoginRequest) (*LoginResult, error)
	Logout(ctx context.Context, sessionID string) error
	GetCurrent(ctx context.Context, sessionID string) (*domain.Session, error)
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
	now := time.Now().UTC()
	sess := &domain.Session{
		SessionID: uuid.NewString(),
		UserID:    u.UserID,
		DeviceID:  device.DeviceID,
		Enable:    true,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.sessionRepo.Put(ctx, sess); err != nil {
		return nil, err
	}
	bearer, err := s.jwtProvider.Sign(u.UserID, device.DeviceID, u.RoleID, sess.SessionID)
	if err != nil {
		return nil, err
	}
	sess.User = u
	return &LoginResult{Bearer: bearer, Session: sess}, nil
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

func (s *service) resolveDevice(ctx context.Context, deviceUUID *string, userID string) (*domain.Device, error) {
	if deviceUUID != nil {
		if d, err := s.deviceRepo.GetByUUID(ctx, *deviceUUID); err == nil {
			return d, nil
		}
	}
	devUUID := uuid.NewString()
	if deviceUUID != nil {
		devUUID = *deviceUUID
	}
	now := time.Now().UTC()
	d := &domain.Device{
		DeviceID:  uuid.NewString(),
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
