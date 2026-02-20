package notification

import (
	"context"
	"fmt"

	"github.com/go-api-nosql/internal/domain"
)

type Service interface {
	ListUnread(ctx context.Context, userID string) ([]domain.Notification, error)
	MarkAsRead(ctx context.Context, notificationID, userID string) (*domain.Notification, error)
}

type notificationStore interface {
	ListUnread(ctx context.Context, userID string) ([]domain.Notification, error)
	Get(ctx context.Context, notificationID string) (*domain.Notification, error)
	MarkAsRead(ctx context.Context, notificationID string) (*domain.Notification, error)
}

type service struct {
	repo notificationStore
}

func NewService(repo notificationStore) Service {
	return &service{repo: repo}
}

func (s *service) ListUnread(ctx context.Context, userID string) ([]domain.Notification, error) {
	return s.repo.ListUnread(ctx, userID)
}

func (s *service) MarkAsRead(ctx context.Context, notificationID, userID string) (*domain.Notification, error) {
	n, err := s.repo.Get(ctx, notificationID)
	if err != nil {
		return nil, err
	}
	if n.UserID != userID {
		return nil, fmt.Errorf("forbidden: %w", domain.ErrForbidden)
	}
	return s.repo.MarkAsRead(ctx, notificationID)
}
