package user

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

type Service interface {
	Register(ctx context.Context, req domain.CreateUserRequest) (*domain.User, error)
	RegisterWithSession(ctx context.Context, req domain.CreateUserRequest) (*domain.Session, string, error)
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
		return nil, errors.New("username already taken")
	}
	if _, err := s.repo.GetByEmail(ctx, req.Email); err == nil {
		return nil, errors.New("email already registered")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	u := &domain.User{
		UserID:       uuid.NewString(),
		Username:     req.Username,
		Email:        req.Email,
		Phone:        req.Phone,
		PasswordHash: string(hash),
		FirstName:    req.FirstName,
		LastName:     req.LastName,
		Birthday:     req.Birthday,
		Enable:       true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := s.repo.Put(ctx, u); err != nil {
		return nil, err
	}
	return u, nil
}

func (s *service) RegisterWithSession(ctx context.Context, req domain.CreateUserRequest) (*domain.Session, string, error) {
	if s.jwtProvider == nil {
		return nil, "", errNotImplemented
	}
	u, err := s.Register(ctx, req)
	if err != nil {
		return nil, "", err
	}
	device, err := s.resolveDevice(ctx, req.DeviceUUID, u.UserID)
	if err != nil {
		return nil, "", err
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
		return nil, "", err
	}
	bearer, err := s.jwtProvider.Sign(u.UserID, device.DeviceID, u.RoleID, sess.SessionID)
	if err != nil {
		return nil, "", err
	}
	sess.User = u
	return sess, bearer, nil
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
		updates["birthday"] = *req.Birthday
	}
	if req.RoleID != nil {
		updates["role_id"] = *req.RoleID
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
