package user

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

type Service interface {
	Register(ctx context.Context, req domain.CreateUserRequest) (*domain.User, error)
	RegisterWithSession(ctx context.Context, req domain.CreateUserRequest) (*domain.Session, string, string, error)
	List(ctx context.Context, limit int, cursor string) ([]domain.User, string, error)
	Get(ctx context.Context, userID string) (*domain.User, error)
	Update(ctx context.Context, userID string, req domain.UpdateUserRequest) (*domain.User, error)
	Delete(ctx context.Context, userID string) error
}

type service struct {
	repo            *dynamo.UserRepo
	sessionRepo     *dynamo.SessionRepo
	deviceRepo      *dynamo.DeviceRepo
	jwtProvider     *jwtinfra.Provider
	refreshTokenDur time.Duration
}

func NewService(repo *dynamo.UserRepo, sessionRepo *dynamo.SessionRepo, deviceRepo *dynamo.DeviceRepo, jwtProvider *jwtinfra.Provider, refreshTokenDur time.Duration) Service {
	return &service{repo: repo, sessionRepo: sessionRepo, deviceRepo: deviceRepo, jwtProvider: jwtProvider, refreshTokenDur: refreshTokenDur}
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
	dev, err := pkgdevice.Resolve(ctx, s.deviceRepo, req.DeviceUUID, u.UserID)
	if err != nil {
		return nil, "", "", err
	}
	refreshToken, err := pkgtoken.NewRefreshToken()
	if err != nil {
		return nil, "", "", err
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
		return nil, "", "", err
	}
	bearer, err := s.jwtProvider.Sign(u.UserID, dev.DeviceID, u.Role, sess.SessionID)
	if err != nil {
		return nil, "", "", err
	}
	sess.User = u
	return sess, bearer, refreshToken, nil
}

func (s *service) List(ctx context.Context, limit int, cursor string) ([]domain.User, string, error) {
	if limit < 1 {
		limit = 50
	}
	return s.repo.ScanPage(ctx, int32(limit), cursor)
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
		switch *req.Role {
		case domain.RoleAdmin, domain.RoleUser:
			updates["role"] = *req.Role
		default:
			return nil, fmt.Errorf("invalid role: %w", domain.ErrBadRequest)
		}
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


