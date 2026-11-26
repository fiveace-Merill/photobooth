package storage

import (
	"context"
	"io"
	"time"
)

// TTLs for presigned URLs. Keep short; they are capability URLs.
const (
	UploadPartTTL = 30 * time.Minute
	GetTTL        = 1 * time.Hour
)

// ObjectStore abstracts object storage so handlers/services can be tested with a
// fake implementation and production can use R2 (S3-compatible).
type ObjectStore interface {
	InitiateMultipart(ctx context.Context, key, contentType string) (uploadID string, err error)
	PresignUploadPart(ctx context.Context, key, uploadID string, partNumber int32, expires time.Duration) (url string, err error)
	ListParts(ctx context.Context, key, uploadID string) ([]UploadedPart, error)
	CompleteMultipart(ctx context.Context, key, uploadID string, parts []UploadedPart) error
	AbortMultipart(ctx context.Context, key, uploadID string) error
	Head(ctx context.Context, key string) (ObjectInfo, error)
	Get(ctx context.Context, key string) (io.ReadCloser, error)
	Put(ctx context.Context, key, contentType string, body io.Reader, size int64) error
	Delete(ctx context.Context, key string) error
	PresignGet(ctx context.Context, key string, expires time.Duration, contentDisposition string) (url string, err error)
}

// UploadedPart identifies a part for multipart completion.
type UploadedPart struct {
	PartNumber int32
	ETag       string
}

// ObjectInfo contains metadata needed by upload completion checks.
type ObjectInfo struct {
	Size int64
}
