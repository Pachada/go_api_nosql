package session

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode"

	"github.com/go-api-nosql/internal/domain"
	pkgdevice "github.com/go-api-nosql/internal/pkg/device"
	"github.com/go-api-nosql/internal/pkg/id"
	pkgtoken "github.com/go-api-nosql/internal/pkg/token"
	"golang.org/x/crypto/bcrypt"
)

// DynamoDB attribute name used in partial update maps.
const fieldEnable = "enable"

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
	LoginWithGoogle(ctx context.Context, credential string, deviceUUID *string) (*LoginResult, error)
	Logout(ctx context.Context, sessionID string) error
	GetCurrent(ctx context.Context, sessionID string) (*domain.Session, error)
	Refresh(ctx context.Context, refreshToken string) (bearer, newRefreshToken string, err error)
}

type sessionStore interface {
	Put(ctx context.Context, s *domain.Session) error
	Get(ctx context.Context, sessionID string) (*domain.Session, error)
	GetByRefreshToken(ctx context.Context, token string) (*domain.Session, error)
	RotateRefreshToken(ctx context.Context, sessionID, newToken string, newExpiry int64) error
	Update(ctx context.Context, sessionID string, updates map[string]interface{}) error
}

type userStore interface {
	GetByUsername(ctx context.Context, username string) (*domain.User, error)
	GetByEmail(ctx context.Context, email string) (*domain.User, error)
	Get(ctx context.Context, userID string) (*domain.User, error)
	Put(ctx context.Context, u *domain.User) error
	Update(ctx context.Context, userID string, updates map[string]interface{}) error
}

type deviceStore interface {
	GetByUUID(ctx context.Context, uuid string) (*domain.Device, error)
	Put(ctx context.Context, d *domain.Device) error
}

type googleVerifier interface {
	Verify(ctx context.Context, token string) (*GooglePayload, error)
}

type GooglePayload struct {
	Sub           string
	Email         string
	EmailVerified bool
	FirstName     string
	LastName      string
}

type jwtSigner interface {
	Sign(userID, deviceID, role, sessionID string) (string, error)
}

type service struct {
	sessionRepo     sessionStore
	userRepo        userStore
	deviceRepo      deviceStore
	jwtProvider     jwtSigner
	googleVerifier  googleVerifier
	refreshTokenDur time.Duration
}

type ServiceDeps struct {
	SessionRepo     sessionStore
	UserRepo        userStore
	DeviceRepo      deviceStore
	JWTProvider     jwtSigner
	GoogleVerifier  googleVerifier
	RefreshTokenDur time.Duration
}

func NewService(deps ServiceDeps) Service {
	return &service{
		sessionRepo:     deps.SessionRepo,
		userRepo:        deps.UserRepo,
		deviceRepo:      deps.DeviceRepo,
		jwtProvider:     deps.JWTProvider,
		googleVerifier:  deps.GoogleVerifier,
		refreshTokenDur: deps.RefreshTokenDur,
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
	if u.Enable == 0 {
		return nil, fmt.Errorf("account disabled: %w", domain.ErrUnauthorized)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(req.Password)); err != nil {
		return nil, fmt.Errorf("invalid credentials: %w", domain.ErrUnauthorized)
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
	return &LoginResult{Bearer: bearer, RefreshToken: refreshToken, Session: sess}, nil
}

func (s *service) Logout(ctx context.Context, sessionID string) error {
	return s.sessionRepo.Update(ctx, sessionID, map[string]interface{}{fieldEnable: false})
}

func (s *service) GetCurrent(ctx context.Context, sessionID string) (*domain.Session, error) {
	sess, err := s.sessionRepo.Get(ctx, sessionID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, fmt.Errorf("session not found: %w", domain.ErrUnauthorized)
		}
		return nil, err
	}
	if !sess.Enable {
		return nil, fmt.Errorf("session expired: %w", domain.ErrUnauthorized)
	}
	u, err := s.userRepo.Get(ctx, sess.UserID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, fmt.Errorf("user not found: %w", domain.ErrUnauthorized)
		}
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
	newToken, err := pkgtoken.NewRefreshToken()
	if err != nil {
		return "", "", err
	}
	newExpiry := time.Now().Add(s.refreshTokenDur).Unix()
	if err := s.sessionRepo.RotateRefreshToken(ctx, sess.SessionID, newToken, newExpiry); err != nil {
		return "", "", err
	}
	u, err := s.userRepo.Get(ctx, sess.UserID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return "", "", fmt.Errorf("user not found: %w", domain.ErrUnauthorized)
		}
		return "", "", err
	}
	bearer, err := s.jwtProvider.Sign(u.UserID, sess.DeviceID, u.Role, sess.SessionID)
	if err != nil {
		return "", "", err
	}
	return bearer, newToken, nil
}

func (s *service) LoginWithGoogle(ctx context.Context, credential string, deviceUUID *string) (*LoginResult, error) {
	payload, err := s.googleVerifier.Verify(ctx, credential)
	if err != nil {
		return nil, err
	}
	if !payload.EmailVerified {
		return nil, fmt.Errorf("google email not verified: %w", domain.ErrUnauthorized)
	}
	if strings.TrimSpace(payload.Email) == "" {
		return nil, fmt.Errorf("google email missing: %w", domain.ErrUnauthorized)
	}
	if payload.Sub == "" {
		return nil, fmt.Errorf("google subject missing: %w", domain.ErrUnauthorized)
	}

	u, err := s.userRepo.GetByEmail(ctx, payload.Email)
	if err != nil {
		if !errors.Is(err, domain.ErrNotFound) {
			return nil, err
		}
		// User does not exist — create one.
		username, err := s.deriveUsername(ctx, payload.Email)
		if err != nil {
			return nil, err
		}
		now := time.Now().UTC()
		u = &domain.User{
			UserID:         id.New(),
			Username:       username,
			Email:          payload.Email,
			FirstName:      payload.FirstName,
			LastName:       payload.LastName,
			AuthProvider:   domain.AuthProviderGoogle,
			GoogleSub:      payload.Sub,
			Role:           domain.RoleUser,
			Enable:         1,
			Verified:       true,
			EmailConfirmed: true,
			CreatedAt:      now,
			UpdatedAt:      now,
		}
		if err := s.userRepo.Put(ctx, u); err != nil {
			return nil, err
		}
	} else {
		if u.Enable == 0 {
			return nil, fmt.Errorf("account disabled: %w", domain.ErrUnauthorized)
		}
		if u.GoogleSub != "" && u.GoogleSub != payload.Sub {
			return nil, fmt.Errorf("google account mismatch: %w", domain.ErrUnauthorized)
		}
		// Link Google sub on first OAuth sign-in for existing accounts.
		// Only allowed if the account has a password set (i.e. self-registered).
		// Admin-provisioned accounts with no password must link explicitly.
		if u.GoogleSub == "" {
			if u.PasswordHash == "" {
				return nil, fmt.Errorf("google linking not allowed for this account: %w", domain.ErrUnauthorized)
			}
			if err := s.userRepo.Update(ctx, u.UserID, map[string]interface{}{
				"google_sub":    payload.Sub,
				"auth_provider": domain.AuthProviderGoogle,
			}); err != nil {
				slog.Warn("failed to link google sub", "user_id", u.UserID, "error", err)
			} else {
				slog.Info("google account linked to existing account", "user_id", u.UserID, "email", payload.Email)
			}
			u.GoogleSub = payload.Sub
			u.AuthProvider = domain.AuthProviderGoogle
		}
	}

	dev, err := pkgdevice.Resolve(ctx, s.deviceRepo, deviceUUID, u.UserID)
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
	return &LoginResult{Bearer: bearer, RefreshToken: refreshToken, Session: sess}, nil
}

// deriveUsername builds a unique username from the email local-part.
func (s *service) deriveUsername(ctx context.Context, email string) (string, error) {
	local := strings.SplitN(email, "@", 2)[0]
	base := sanitizeUsername(local)
	if base == "" {
		base = "user"
	}
	candidate := base
	for i := 1; i <= 99; i++ {
		_, err := s.userRepo.GetByUsername(ctx, candidate)
		if errors.Is(err, domain.ErrNotFound) {
			return candidate, nil
		}
		if err != nil {
			return "", err
		}
		candidate = fmt.Sprintf("%s%d", base, i)
	}
	// Final check after loop exhaustion.
	_, err := s.userRepo.GetByUsername(ctx, candidate)
	if errors.Is(err, domain.ErrNotFound) {
		return candidate, nil
	}
	return "", fmt.Errorf("unable to derive unique username from %q: %w", base, domain.ErrConflict)
}

// sanitizeUsername keeps only lowercase letters, digits, dots, underscores, and hyphens.
func sanitizeUsername(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '.' || r == '_' || r == '-' {
			b.WriteRune(r)
		}
	}
	return b.String()
}
