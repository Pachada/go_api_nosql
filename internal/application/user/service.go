package user

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/go-api-nosql/internal/domain"
	"github.com/go-api-nosql/internal/infrastructure/dynamo"
	jwtinfra "github.com/go-api-nosql/internal/infrastructure/jwt"
	"github.com/go-api-nosql/internal/pkg/id"
	"golang.org/x/crypto/bcrypt"
)

type Service interface {
	Register(ctx context.Context, req domain.CreateUserRequest) (*domain.User, error)
	RegisterWithSession(ctx context.Context, req domain.CreateUserRequest) (*domain.Session, string, string, error)
	List(ctx context.Context, page, perPage int) ([]domain.User, int, error)
	Get(ctx context.Context, userID string) (*domain.User, error)
	Update(ctx context.Context, userID string, req domain.UpdateUserRequest) (*domain.User, error)
	Delete(ctx context.Context, userID string) error
}

type service struct {
	repo        *dynamo.UserRepo
	sessionRepo *dynamo.SessionRepo
	deviceRepo  *dynamo.DeviceRepo
	jwtProvider *jwtinfra.Provider
}

func NewService(repo *dynamo.UserRepo, sessionRepo *dynamo.SessionRepo, deviceRepo *dynamo.DeviceRepo, jwtProvider *jwtinfra.Provider) Service {
	return &service{repo: repo, sessionRepo: sessionRepo, deviceRepo: deviceRepo, jwtProvider: jwtProvider}
}

func (s *service) Register(ctx context.Context, req domain.CreateUserRequest) (*domain.User, error) {
	if _, err := s.repo.GetByUsername(ctx, req.Username); err == nil {
		return nil, fmt.Errorf("username already taken: %w", domain.ErrConflict)
	}
	if _, err := s.repo.GetByEmail(ctx, req.Email); err == nil {
		return nil, fmt.Errorf("email already registered: %w", domain.ErrConflict)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	var birthday time.Time
	if req.Birthday != "" {
		birthday, err = time.Parse("2006-01-02", req.Birthday)
		if err != nil {
			return nil, fmt.Errorf("birthday must be in YYYY-MM-DD format: %w", domain.ErrBadRequest)
		}
	}
	now := time.Now().UTC()
	u := &domain.User{
		UserID:       id.New(),
		Username:     req.Username,
		Email:        req.Email,
		Phone:        req.Phone,
		PasswordHash: string(hash),
		FirstName:    req.FirstName,
		LastName:     req.LastName,
		Birthday:     birthday,
		Role:         domain.RoleUser,
		Enable:       true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := s.repo.Put(ctx, u); err != nil {
		return nil, err
	}
	return u, nil
}

func (s *service) RegisterWithSession(ctx context.Context, req domain.CreateUserRequest) (*domain.Session, string, string, error) {
	if s.jwtProvider == nil {
		return nil, "", "", errNotImplemented
	}
	u, err := s.Register(ctx, req)
	if err != nil {
		return nil, "", "", err
	}
	device, err := s.resolveDevice(ctx, req.DeviceUUID, u.UserID)
	if err != nil {
		return nil, "", "", err
	}
	refreshToken := newRefreshToken()
	now := time.Now().UTC()
	sess := &domain.Session{
		SessionID:        id.New(),
		UserID:           u.UserID,
		DeviceID:         device.DeviceID,
		Enable:           true,
		RefreshToken:     refreshToken,
		RefreshExpiresAt: now.Add(30 * 24 * time.Hour).Unix(),
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := s.sessionRepo.Put(ctx, sess); err != nil {
		return nil, "", "", err
	}
	bearer, err := s.jwtProvider.Sign(u.UserID, device.DeviceID, u.Role, sess.SessionID)
	if err != nil {
		return nil, "", "", err
	}
	sess.User = u
	return sess, bearer, refreshToken, nil
}

func (s *service) List(ctx context.Context, page, perPage int) ([]domain.User, int, error) {
	all, err := s.repo.Scan(ctx)
	if err != nil {
		return nil, 0, err
	}
	total := len(all)
	start := (page - 1) * perPage
	if start >= total {
		return []domain.User{}, total, nil
	}
	end := start + perPage
	if end > total {
		end = total
	}
	return all[start:end], total, nil
}

func (s *service) Get(ctx context.Context, userID string) (*domain.User, error) {
	return s.repo.Get(ctx, userID)
}

func (s *service) Update(ctx context.Context, userID string, req domain.UpdateUserRequest) (*domain.User, error) {
	updates := map[string]interface{}{}
	if req.Username != nil {
		updates["username"] = *req.Username
	}
	if req.Email != nil {
		updates["email"] = *req.Email
	}
	if req.Phone != nil {
		updates["phone"] = *req.Phone
	}
	if req.FirstName != nil {
		updates["first_name"] = *req.FirstName
	}
	if req.LastName != nil {
		updates["last_name"] = *req.LastName
	}
	if req.Birthday != nil {
		t, err := time.Parse("2006-01-02", *req.Birthday)
		if err != nil {
			return nil, fmt.Errorf("birthday must be in YYYY-MM-DD format: %w", domain.ErrBadRequest)
		}
		updates["birthday"] = t
	}
	if req.Role != nil {
		updates["role"] = *req.Role
	}
	if req.Enable != nil {
		updates["enable"] = *req.Enable
	}
	if len(updates) == 0 {
		return s.repo.Get(ctx, userID)
	}
	if err := s.repo.Update(ctx, userID, updates); err != nil {
		return nil, err
	}
	return s.repo.Get(ctx, userID)
}

func (s *service) Delete(ctx context.Context, userID string) error {
	if err := s.repo.SoftDelete(ctx, userID); err != nil {
		return err
	}
	return s.sessionRepo.SoftDeleteByUser(ctx, userID)
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
