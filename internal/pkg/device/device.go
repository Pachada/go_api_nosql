package device

import (
	"context"
	"time"

	"github.com/go-api-nosql/internal/domain"
	"github.com/go-api-nosql/internal/infrastructure/dynamo"
	"github.com/go-api-nosql/internal/pkg/id"
)

// Resolve returns the existing Device for deviceUUID when found, otherwise
// creates a new one associated with userID and persists it.
func Resolve(ctx context.Context, repo *dynamo.DeviceRepo, deviceUUID *string, userID string) (*domain.Device, error) {
	if deviceUUID != nil {
		if d, err := repo.GetByUUID(ctx, *deviceUUID); err == nil {
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
	if err := repo.Put(ctx, d); err != nil {
		return nil, err
	}
	return d, nil
}
