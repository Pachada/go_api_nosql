package role

import (
	"context"

	"github.com/go-api-nosql/internal/domain"
	"github.com/go-api-nosql/internal/infrastructure/dynamo"
	"github.com/google/uuid"
)

type Service interface {
	List(ctx context.Context) ([]domain.Role, error)
	Get(ctx context.Context, roleID string) (*domain.Role, error)
	Create(ctx context.Context, input domain.RoleInput) (*domain.Role, error)
	Update(ctx context.Context, roleID string, input domain.RoleInput) (*domain.Role, error)
	Delete(ctx context.Context, roleID string) error
}

type service struct {
	repo *dynamo.RoleRepo
}

func NewService(repo *dynamo.RoleRepo) Service {
	return &service{repo: repo}
}

func (s *service) List(ctx context.Context) ([]domain.Role, error) {
	return s.repo.Scan(ctx)
}

func (s *service) Get(ctx context.Context, roleID string) (*domain.Role, error) {
	return s.repo.Get(ctx, roleID)
}

func (s *service) Create(ctx context.Context, input domain.RoleInput) (*domain.Role, error) {
	role := &domain.Role{
		RoleID: uuid.NewString(),
		Name:   input.Name,
		Enable: true,
	}
	if err := s.repo.Put(ctx, role); err != nil {
		return nil, err
	}
	return role, nil
}

func (s *service) Update(ctx context.Context, roleID string, input domain.RoleInput) (*domain.Role, error) {
	updates := map[string]interface{}{"name": input.Name}
	if input.Enable != nil {
		updates["enable"] = *input.Enable
	}
	if err := s.repo.Update(ctx, roleID, updates); err != nil {
		return nil, err
	}
	return s.repo.Get(ctx, roleID)
}

func (s *service) Delete(ctx context.Context, roleID string) error {
	return s.repo.Update(ctx, roleID, map[string]interface{}{"enable": false})
}
