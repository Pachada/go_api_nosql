package status

import (
	"context"

	"github.com/go-api-nosql/internal/domain"
	"github.com/go-api-nosql/internal/pkg/id"
)

// DynamoDB attribute name used in partial update maps.
const fieldDescription = "description"

type Service interface {
	List(ctx context.Context) ([]domain.Status, error)
	Get(ctx context.Context, statusID string) (*domain.Status, error)
	Create(ctx context.Context, input domain.StatusInput) (*domain.Status, error)
	Update(ctx context.Context, statusID string, input domain.StatusInput) (*domain.Status, error)
	Delete(ctx context.Context, statusID string) error // hard delete
}

type statusStore interface {
	Scan(ctx context.Context) ([]domain.Status, error)
	Get(ctx context.Context, statusID string) (*domain.Status, error)
	Put(ctx context.Context, s *domain.Status) error
	Update(ctx context.Context, statusID string, updates map[string]interface{}) error
	HardDelete(ctx context.Context, statusID string) error
}

type service struct {
	repo statusStore
}

func NewService(repo statusStore) Service {
	return &service{repo: repo}
}

func (s *service) List(ctx context.Context) ([]domain.Status, error) {
	return s.repo.Scan(ctx)
}

func (s *service) Get(ctx context.Context, statusID string) (*domain.Status, error) {
	return s.repo.Get(ctx, statusID)
}

func (s *service) Create(ctx context.Context, input domain.StatusInput) (*domain.Status, error) {
	st := &domain.Status{
		StatusID:    id.New(),
		Description: input.Description,
	}
	if err := s.repo.Put(ctx, st); err != nil {
		return nil, err
	}
	return st, nil
}

func (s *service) Update(ctx context.Context, statusID string, input domain.StatusInput) (*domain.Status, error) {
	if err := s.repo.Update(ctx, statusID, map[string]interface{}{fieldDescription: input.Description}); err != nil {
		return nil, err
	}
	return s.repo.Get(ctx, statusID)
}

func (s *service) Delete(ctx context.Context, statusID string) error {
	return s.repo.HardDelete(ctx, statusID)
}
