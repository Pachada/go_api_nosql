package http

import (
	"context"
	"io"

	"github.com/go-api-nosql/internal/domain"
)

// UserRepository is the minimal interface the router requires from a user store.
type UserRepository interface {
	GetByUsername(ctx context.Context, username string) (*domain.User, error)
	GetByEmail(ctx context.Context, email string) (*domain.User, error)
	Put(ctx context.Context, u *domain.User) error
	// QueryPage returns a page of enabled users via the `enable-index` GSI.
	// Only users with enable=1 are returned; this is not a full table scan.
	QueryPage(ctx context.Context, limit int32, cursor string) ([]domain.User, string, error)
	Get(ctx context.Context, userID string) (*domain.User, error)
	Update(ctx context.Context, userID string, updates map[string]interface{}) error
	SoftDelete(ctx context.Context, userID string) error
}

// SessionRepository is the minimal interface the router requires from a session store.
type SessionRepository interface {
	Put(ctx context.Context, s *domain.Session) error
	Get(ctx context.Context, sessionID string) (*domain.Session, error)
	GetByRefreshToken(ctx context.Context, token string) (*domain.Session, error)
	RotateRefreshToken(ctx context.Context, sessionID, newToken string, newExpiry int64) error
	Update(ctx context.Context, sessionID string, updates map[string]interface{}) error
	SoftDeleteByUser(ctx context.Context, userID string) error
}

// DeviceRepository is the minimal interface the router requires from a device store.
type DeviceRepository interface {
	GetByUUID(ctx context.Context, uuid string) (*domain.Device, error)
	Put(ctx context.Context, d *domain.Device) error
	ListByUser(ctx context.Context, userID string) ([]domain.Device, error)
	Get(ctx context.Context, deviceID string) (*domain.Device, error)
	Update(ctx context.Context, deviceID string, updates map[string]interface{}) error
	SoftDelete(ctx context.Context, deviceID string) error
}

// StatusRepository is the minimal interface the router requires from a status store.
type StatusRepository interface {
	Scan(ctx context.Context) ([]domain.Status, error)
	Get(ctx context.Context, statusID string) (*domain.Status, error)
	Put(ctx context.Context, s *domain.Status) error
	Update(ctx context.Context, statusID string, updates map[string]interface{}) error
	HardDelete(ctx context.Context, statusID string) error
}

// NotificationRepository is the minimal interface the router requires from a notification store.
type NotificationRepository interface {
	ListUnread(ctx context.Context, userID string) ([]domain.Notification, error)
	Get(ctx context.Context, notificationID string) (*domain.Notification, error)
	MarkAsRead(ctx context.Context, notificationID string) (*domain.Notification, error)
}

// FileRepository is the minimal interface the router requires from a file store.
type FileRepository interface {
	Put(ctx context.Context, f *domain.File) error
	Get(ctx context.Context, fileID string) (*domain.File, error)
	SoftDelete(ctx context.Context, fileID string) error
}

// VerificationRepository is the minimal interface the router requires from a verification store.
type VerificationRepository interface {
	Put(ctx context.Context, v *domain.UserVerification) error
	Get(ctx context.Context, userID, verType string) (*domain.UserVerification, error)
	Delete(ctx context.Context, userID, verType string) error
}

// AppVersionRepository is the minimal interface the router requires from an app-version store.
type AppVersionRepository interface {
	GetLatest(ctx context.Context) (*domain.AppVersion, error)
}

// ObjectStore is the minimal interface the router requires from an object storage backend.
type ObjectStore interface {
	Upload(ctx context.Context, key string, r io.Reader, contentType string) (string, error)
	Download(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
}
