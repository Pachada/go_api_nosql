package user

import (
	"context"
	"fmt"
	"time"

	"github.com/go-api-nosql/internal/domain"
	pkgdevice "github.com/go-api-nosql/internal/pkg/device"
	"github.com/go-api-nosql/internal/pkg/id"
	pkgtoken "github.com/go-api-nosql/internal/pkg/token"
	"golang.org/x/crypto/bcrypt"
)

// DynamoDB attribute names used in partial update maps.
const (
	fieldUsername     = "username"
	fieldEmail        = "email"
	fieldPhone        = "phone"
	fieldFirstName    = "first_name"
	fieldLastName     = "last_name"
	fieldBirthday     = "birthday"
	fieldRole         = "role"
	fieldEnable       = "enable"
	fieldPasswordHash = "password_hash"
)

type Service interface {
	Register(ctx context.Context, req domain.CreateUserRequest) (*domain.User, error)
	RegisterWithSession(ctx context.Context, req domain.CreateUserRequest) (*domain.Session, string, string, error)
	List(ctx context.Context, limit int, cursor string) ([]domain.User, string, error)
	Get(ctx context.Context, userID string) (*domain.User, error)
	Update(ctx context.Context, userID string, req domain.UpdateUserRequest) (*domain.User, error)
	Delete(ctx context.Context, userID string) error
	ChangePassword(ctx context.Context, userID, currentPassword, newPassword string) error
}

type userStore interface {
	GetByUsername(ctx context.Context, username string) (*domain.User, error)
	GetByEmail(ctx context.Context, email string) (*domain.User, error)
	Put(ctx context.Context, u *domain.User) error
	QueryPage(ctx context.Context, limit int32, cursor string) ([]domain.User, string, error)
	Get(ctx context.Context, userID string) (*domain.User, error)
	Update(ctx context.Context, userID string, updates map[string]interface{}) error
	SoftDelete(ctx context.Context, userID string) error
}

type sessionStore interface {
	Put(ctx context.Context, s *domain.Session) error
	SoftDeleteByUser(ctx context.Context, userID string) error
}

type deviceStore interface {
	GetByUUID(ctx context.Context, uuid string) (*domain.Device, error)
	Put(ctx context.Context, d *domain.Device) error
}

type jwtSigner interface {
	Sign(userID, deviceID, role, sessionID string) (string, error)
}

type service struct {
	repo            userStore
	sessionRepo     sessionStore
	deviceRepo      deviceStore
	jwtProvider     jwtSigner
	refreshTokenDur time.Duration
}

type ServiceDeps struct {
	UserRepo        userStore
	SessionRepo     sessionStore
	DeviceRepo      deviceStore
	JWTProvider     jwtSigner
	RefreshTokenDur time.Duration
}

func NewService(deps ServiceDeps) Service {
	return &service{
		repo:            deps.UserRepo,
		sessionRepo:     deps.SessionRepo,
		deviceRepo:      deps.DeviceRepo,
		jwtProvider:     deps.JWTProvider,
		refreshTokenDur: deps.RefreshTokenDur,
	}
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
		Enable:       1,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := s.repo.Put(ctx, u); err != nil {
		return nil, err
	}
	return u, nil
}

func (s *service) RegisterWithSession(ctx context.Context, req domain.CreateUserRequest) (*domain.Session, string, string, error) {
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
	return s.repo.QueryPage(ctx, int32(limit), cursor)
}

func (s *service) Get(ctx context.Context, userID string) (*domain.User, error) {
	return s.repo.Get(ctx, userID)
}

func (s *service) Update(ctx context.Context, userID string, req domain.UpdateUserRequest) (*domain.User, error) {
	updates := map[string]interface{}{}
	if req.Username != nil {
		updates[fieldUsername] = *req.Username
	}
	if req.Email != nil {
		updates[fieldEmail] = *req.Email
	}
	if req.Phone != nil {
		updates[fieldPhone] = *req.Phone
	}
	if req.FirstName != nil {
		updates[fieldFirstName] = *req.FirstName
	}
	if req.LastName != nil {
		updates[fieldLastName] = *req.LastName
	}
	if req.Birthday != nil {
		t, err := time.Parse("2006-01-02", *req.Birthday)
		if err != nil {
			return nil, fmt.Errorf("birthday must be in YYYY-MM-DD format: %w", domain.ErrBadRequest)
		}
		updates[fieldBirthday] = t
	}
	if req.Role != nil {
		switch *req.Role {
		case domain.RoleAdmin, domain.RoleUser:
			updates[fieldRole] = *req.Role
		default:
			return nil, fmt.Errorf("invalid role: %w", domain.ErrBadRequest)
		}
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

func (s *service) ChangePassword(ctx context.Context, userID, currentPassword, newPassword string) error {
	u, err := s.repo.Get(ctx, userID)
	if err != nil {
		return err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(currentPassword)); err != nil {
		return fmt.Errorf("current password is incorrect: %w", domain.ErrUnauthorized)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return s.repo.Update(ctx, userID, map[string]interface{}{fieldPasswordHash: string(hash)})
}
