package file

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"path"
	"strings"
	"time"

	"github.com/go-api-nosql/internal/domain"
	"github.com/go-api-nosql/internal/infrastructure/dynamo"
	s3infra "github.com/go-api-nosql/internal/infrastructure/s3"
	"github.com/go-api-nosql/internal/pkg/id"
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
	safeName := sanitizeFilename(input.Filename)
	key := fmt.Sprintf("files/%s/%s", input.UploaderID, safeName)
	hasher := sha256.New()
	tee := io.TeeReader(input.Reader, hasher)
	if _, err := s.s3.Upload(ctx, key, tee, input.ContentType); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	f := &domain.File{
		FileID:           id.New(),
		Object:           key,
		Size:             input.Size,
		Type:             input.ContentType,
		Name:             safeName,
		Hash:             hex.EncodeToString(hasher.Sum(nil)),
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
	safeName := sanitizeFilename(filename)
	key := fmt.Sprintf("files/%s/%s", uploaderID, safeName)
	decoded, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		return nil, fmt.Errorf("decode base64: %w", domain.ErrBadRequest)
	}
	contentType := contentTypeFromName(safeName)
	if _, err := s.s3.Upload(ctx, key, bytes.NewReader(decoded), contentType); err != nil {
		return nil, err
	}
	sum := sha256.Sum256(decoded)
	now := time.Now().UTC()
	f := &domain.File{
		FileID:           id.New(),
		Object:           key,
		Size:             int64(len(decoded)),
		Type:             contentType,
		Name:             safeName,
		Hash:             hex.EncodeToString(sum[:]),
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
		return nil, nil, fmt.Errorf("file not found: %w", domain.ErrNotFound)
	}
	if f.IsPrivate && f.UploadedByUserID != requesterID && !isAdmin {
		return nil, nil, fmt.Errorf("access denied: %w", domain.ErrForbidden)
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
		return fmt.Errorf("file not found: %w", domain.ErrNotFound)
	}
	if f.IsPrivate && f.UploadedByUserID != requesterID && !isAdmin {
		return fmt.Errorf("access denied: %w", domain.ErrForbidden)
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

// sanitizeFilename strips directory components and keeps only safe characters
// (alphanumeric, dot, dash, underscore) to prevent path traversal in S3 keys.
func sanitizeFilename(name string) string {
	name = path.Base(name) // drop any leading path components / traversal sequences
	var b strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || r == '.' || r == '-' || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteRune('_')
		}
	}
	if result := b.String(); result != "" && result != "." {
		return result
	}
	return "_"
}
