package device

import (
	"context"
	"errors"
	"time"

	"github.com/go-api-nosql/internal/domain"
	"github.com/go-api-nosql/internal/pkg/id"
)

type deviceStorer interface {
	GetByUUID(ctx context.Context, uuid string) (*domain.Device, error)
	Put(ctx context.Context, d *domain.Device) error
}

// Resolve returns the existing Device for deviceUUID when found, otherwise
// creates a new one associated with userID and persists it.
func Resolve(ctx context.Context, repo deviceStorer, deviceUUID *string, userID string) (*domain.Device, error) {
	if deviceUUID != nil {
		d, err := repo.GetByUUID(ctx, *deviceUUID)
		if err == nil {
			return d, nil
		}
		if !errors.Is(err, domain.ErrNotFound) {
			return nil, err
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
	if err := repo.Put(ctx, d); err != nil {
		return nil, err
	}
	return d, nil
}
