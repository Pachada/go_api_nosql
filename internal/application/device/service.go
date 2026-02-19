package device

import (
	"context"
	"strconv"

	"github.com/go-api-nosql/internal/domain"
)

type Service interface {
	List(ctx context.Context, userID string) ([]domain.Device, error)
	Get(ctx context.Context, deviceID string) (*domain.Device, error)
	Update(ctx context.Context, deviceID string, req domain.UpdateDeviceRequest) (*domain.Device, error)
	Delete(ctx context.Context, deviceID string) error
	// CheckVersion returns true if version is up to date, false if update required.
	CheckVersion(ctx context.Context, sessionID string, version float64) (bool, error)
}

type deviceStore interface {
	ListByUser(ctx context.Context, userID string) ([]domain.Device, error)
	Get(ctx context.Context, deviceID string) (*domain.Device, error)
	Update(ctx context.Context, deviceID string, updates map[string]interface{}) error
	SoftDelete(ctx context.Context, deviceID string) error
}

type appVersionStore interface {
	GetLatest(ctx context.Context) (*domain.AppVersion, error)
}

type service struct {
	repo           deviceStore
	appVersionRepo appVersionStore
}

func NewService(repo deviceStore, appVersionRepo appVersionStore) Service {
	return &service{repo: repo, appVersionRepo: appVersionRepo}
}

func (s *service) List(ctx context.Context, userID string) ([]domain.Device, error) {
	return s.repo.ListByUser(ctx, userID)
}

func (s *service) Get(ctx context.Context, deviceID string) (*domain.Device, error) {
	return s.repo.Get(ctx, deviceID)
}

func (s *service) Update(ctx context.Context, deviceID string, req domain.UpdateDeviceRequest) (*domain.Device, error) {
	updates := map[string]interface{}{}
	if req.Token != nil {
		updates["token"] = *req.Token
	}
	if req.AppVersionID != nil {
		updates["app_version_id"] = *req.AppVersionID
	}
	if len(updates) == 0 {
		return s.repo.Get(ctx, deviceID)
	}
	if err := s.repo.Update(ctx, deviceID, updates); err != nil {
		return nil, err
	}
	return s.repo.Get(ctx, deviceID)
}

func (s *service) Delete(ctx context.Context, deviceID string) error {
	return s.repo.SoftDelete(ctx, deviceID)
}

func (s *service) CheckVersion(ctx context.Context, _ string, version float64) (bool, error) {
	latest, err := s.appVersionRepo.GetLatest(ctx)
	if err != nil {
		// No version on record — pass.
		return true, nil
	}
	latestF, err := strconv.ParseFloat(latest.Version, 64)
	if err != nil {
		return true, nil
	}
	return version >= latestF, nil
}
