package file

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/go-api-nosql/internal/domain"
	"github.com/go-api-nosql/internal/infrastructure/dynamo"
	s3infra "github.com/go-api-nosql/internal/infrastructure/s3"
	"github.com/google/uuid"
)

type UploadInput struct {
	Reader      io.Reader
	Filename    string
	ContentType string
	Size        int64
	IsPrivate   bool
	IsThumbnail bool
	UploaderID  string
}

type Service interface {
	Upload(ctx context.Context, input UploadInput) (*domain.File, error)
	UploadBase64(ctx context.Context, filename, base64Data string, uploaderID string) (*domain.File, error)
	Download(ctx context.Context, fileID, requesterID string, isAdmin bool) (io.ReadCloser, *domain.File, error)
	Delete(ctx context.Context, fileID, requesterID string, isAdmin bool) error
	GetBase64(ctx context.Context, fileID, requesterID string, isAdmin bool) (*domain.File, string, error)
}

type service struct {
	s3       *s3infra.Store
	fileRepo *dynamo.FileRepo
}

func NewService(s3 *s3infra.Store, fileRepo *dynamo.FileRepo) Service {
	return &service{s3: s3, fileRepo: fileRepo}
}

func (s *service) Upload(ctx context.Context, input UploadInput) (*domain.File, error) {
	key := fmt.Sprintf("files/%s/%s", input.UploaderID, input.Filename)
	if _, err := s.s3.Upload(ctx, key, input.Reader, input.ContentType); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	f := &domain.File{
		FileID:           uuid.NewString(),
		Object:           key,
		Size:             input.Size,
		Type:             input.ContentType,
		Name:             input.Filename,
		Hash:             "",
		IsThumbnail:      btoi(input.IsThumbnail),
		IsPrivate:        input.IsPrivate,
		UploadedByUserID: input.UploaderID,
		Enable:           true,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := s.fileRepo.Put(ctx, f); err != nil {
		return nil, err
	}
	return f, nil
}

func (s *service) UploadBase64(ctx context.Context, filename, base64Data string, uploaderID string) (*domain.File, error) {
	key := fmt.Sprintf("files/%s/%s", uploaderID, filename)
	if _, err := s.s3.UploadBase64(ctx, key, base64Data); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	f := &domain.File{
		FileID:           uuid.NewString(),
		Object:           key,
		Size:             0,
		Type:             contentTypeFromName(filename),
		Name:             filename,
		Hash:             "",
		IsThumbnail:      0,
		IsPrivate:        false,
		UploadedByUserID: uploaderID,
		Enable:           true,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := s.fileRepo.Put(ctx, f); err != nil {
		return nil, err
	}
	return f, nil
}

func (s *service) Download(ctx context.Context, fileID, requesterID string, isAdmin bool) (io.ReadCloser, *domain.File, error) {
	f, err := s.fileRepo.Get(ctx, fileID)
	if err != nil {
		return nil, nil, err
	}
	if !f.Enable {
		return nil, nil, errors.New("file not found")
	}
	if f.IsPrivate && f.UploadedByUserID != requesterID && !isAdmin {
		return nil, nil, errors.New("access denied")
	}
	rc, err := s.s3.Download(ctx, f.Object)
	if err != nil {
		return nil, nil, err
	}
	return rc, f, nil
}

func (s *service) Delete(ctx context.Context, fileID, requesterID string, isAdmin bool) error {
	f, err := s.fileRepo.Get(ctx, fileID)
	if err != nil {
		return err
	}
	if !f.Enable {
		return errors.New("file not found")
	}
	if f.IsPrivate && f.UploadedByUserID != requesterID && !isAdmin {
		return errors.New("access denied")
	}
	if err := s.s3.Delete(ctx, f.Object); err != nil {
		return err
	}
	return s.fileRepo.SoftDelete(ctx, fileID)
}

func (s *service) GetBase64(ctx context.Context, fileID, requesterID string, isAdmin bool) (*domain.File, string, error) {
	rc, f, err := s.Download(ctx, fileID, requesterID, isAdmin)
	if err != nil {
		return nil, "", err
	}
	defer rc.Close()
	data, err := io.ReadAll(rc)
	if err != nil {
		return nil, "", err
	}
	return f, base64.StdEncoding.EncodeToString(data), nil
}

func btoi(b bool) int {
	if b {
		return 1
	}
	return 0
}

func contentTypeFromName(filename string) string {
	lower := strings.ToLower(filename)
	switch {
	case strings.HasSuffix(lower, ".jpg") || strings.HasSuffix(lower, ".jpeg"):
		return "image/jpeg"
	case strings.HasSuffix(lower, ".png"):
		return "image/png"
	case strings.HasSuffix(lower, ".pdf"):
		return "application/pdf"
	default:
		return "application/octet-stream"
	}
}
