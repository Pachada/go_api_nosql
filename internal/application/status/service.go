package status

import (
	"context"

	"github.com/go-api-nosql/internal/domain"
	"github.com/go-api-nosql/internal/infrastructure/dynamo"
	"github.com/google/uuid"
)

type Service interface {
	List(ctx context.Context) ([]domain.Status, error)
	Get(ctx context.Context, statusID string) (*domain.Status, error)
	Create(ctx context.Context, input domain.StatusInput) (*domain.Status, error)
	Update(ctx context.Context, statusID string, input domain.StatusInput) (*domain.Status, error)
	Delete(ctx context.Context, statusID string) error // hard delete
}

type service struct {
	repo *dynamo.StatusRepo
}

func NewService(repo *dynamo.StatusRepo) Service {
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
		StatusID:    uuid.NewString(),
		Description: input.Description,
	}
	if err := s.repo.Put(ctx, st); err != nil {
		return nil, err
	}
	return st, nil
}

func (s *service) Update(ctx context.Context, statusID string, input domain.StatusInput) (*domain.Status, error) {
	if err := s.repo.Update(ctx, statusID, map[string]interface{}{"description": input.Description}); err != nil {
		return nil, err
	}
	return s.repo.Get(ctx, statusID)
}

func (s *service) Delete(ctx context.Context, statusID string) error {
	return s.repo.HardDelete(ctx, statusID)
}
