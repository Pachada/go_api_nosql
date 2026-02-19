package notification

import (
	"context"

	"github.com/go-api-nosql/internal/domain"
	"github.com/go-api-nosql/internal/infrastructure/dynamo"
)

type Service interface {
	ListUnread(ctx context.Context, userID string) ([]domain.Notification, error)
	Get(ctx context.Context, notificationID, userID string) (*domain.Notification, error)
	MarkAsRead(ctx context.Context, notificationID string) (*domain.Notification, error)
}

type service struct {
	repo *dynamo.NotificationRepo
}

func NewService(repo *dynamo.NotificationRepo) Service {
	return &service{repo: repo}
}

func (s *service) ListUnread(ctx context.Context, userID string) ([]domain.Notification, error) {
	return s.repo.ListUnread(ctx, userID)
}

func (s *service) Get(ctx context.Context, notificationID, _ string) (*domain.Notification, error) {
	return s.repo.Get(ctx, notificationID)
}

func (s *service) MarkAsRead(ctx context.Context, notificationID string) (*domain.Notification, error) {
	return s.repo.MarkAsRead(ctx, notificationID)
}
