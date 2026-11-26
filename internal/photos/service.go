package photos

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	"github.com/fiveace-Merill/photobooth/internal/photos/store"
	"github.com/fiveace-Merill/photobooth/internal/platform/database"
	photostorage "github.com/fiveace-Merill/photobooth/internal/storage"
)

const (
	partSizeBytes   int64 = 10 << 20
	maxPresignParts       = 20
)

type Service struct {
	pool      *pgxpool.Pool
	objects   photostorage.ObjectStore
	river     *river.Client[pgx.Tx]
	maxBytes  int64
	maxPhotos int
	maxMP     int
	logger    *slog.Logger
}

type Deps struct {
	Pool               *pgxpool.Pool
	Objects            photostorage.ObjectStore
	River              *river.Client[pgx.Tx]
	MaxPhotoBytes      int64
	MaxPhotosPerShoot  int
	MaxImageMegapixels int
	Logger             *slog.Logger
}

func NewService(d Deps) *Service {
	if d.Logger == nil {
		d.Logger = slog.Default()
	}
	return &Service{pool: d.Pool, objects: d.Objects, river: d.River, maxBytes: d.MaxPhotoBytes, maxPhotos: d.MaxPhotosPerShoot, maxMP: d.MaxImageMegapixels, logger: d.Logger}
}

type CreateUploadInput struct {
	Filename    string `json:"filename"`
	ContentType string `json:"contentType"`
	SizeBytes   int64  `json:"sizeBytes"`
}
type UploadPartURL struct {
	PartNumber int32     `json:"partNumber"`
	URL        string    `json:"url"`
	ExpiresAt  time.Time `json:"expiresAt"`
}
type CreateUploadResult struct {
	PhotoID       uuid.UUID       `json:"photoId"`
	ShootID       uuid.UUID       `json:"shootId"`
	PartSizeBytes int64           `json:"partSizeBytes"`
	PartCount     int32           `json:"partCount"`
	UploadParts   []UploadPartURL `json:"uploadParts"`
	Status        string          `json:"status"`
}

type Photo struct {
	ID          uuid.UUID  `json:"id"`
	ShootID     uuid.UUID  `json:"shootId"`
	Filename    string     `json:"filename"`
	ContentType string     `json:"contentType"`
	SizeBytes   int64      `json:"sizeBytes"`
	Width       *int32     `json:"width,omitempty"`
	Height      *int32     `json:"height,omitempty"`
	Status      string     `json:"status"`
	Error       *string    `json:"error,omitempty"`
	ProcessedAt *time.Time `json:"processedAt,omitempty"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
	SortKey     int32      `json:"sortKey"`
	ThumbURL    *string    `json:"thumbUrl,omitempty"`
}

type ListResult struct {
	Items  []Photo `json:"items"`
	Totals struct {
		Count int64 `json:"count"`
		Bytes int64 `json:"bytes"`
	} `json:"totals"`
	Quota struct {
		UsedBytes  int64 `json:"usedBytes"`
		QuotaBytes int64 `json:"quotaBytes"`
	} `json:"quota"`
}

type serviceError struct {
	code    string
	message string
	status  int
}

func (e *serviceError) Error() string { return e.message }
func errCode(status int, code, msg string) error {
	return &serviceError{status: status, code: code, message: msg}
}
func AsServiceError(err error) (status int, code, message string, ok bool) {
	var se *serviceError
	if errors.As(err, &se) {
		return se.status, se.code, se.message, true
	}
	return 0, "", "", false
}

func (s *Service) CreateUpload(ctx context.Context, photographerID, shootID uuid.UUID, in CreateUploadInput) (CreateUploadResult, error) {
	in.Filename = sanitizeFilename(in.Filename)
	if err := validateContentType(in.ContentType); err != nil {
		return CreateUploadResult{}, errCode(415, "unsupported_type", "Unsupported file type")
	}
	if in.SizeBytes <= 0 || in.SizeBytes > s.maxBytes {
		return CreateUploadResult{}, errCode(413, "file_too_large", "File too large")
	}
	partCount := int32((in.SizeBytes + partSizeBytes - 1) / partSizeBytes)
	if partCount <= 0 {
		partCount = 1
	}

	var photoID uuid.UUID
	var uploadID string
	var key string
	err := database.WithTenant(ctx, s.pool, pgUUID(photographerID), func(tx pgx.Tx) error {
		q := store.New(tx)
		shoot, err := q.GetShootForPhoto(ctx, store.GetShootForPhotoParams{PhotographerID: pgUUID(photographerID), ID: pgUUID(shootID)})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return errCode(404, "not_found", "Shoot not found")
			}
			return err
		}
		if shoot.Status == "expired" {
			return errCode(409, "shoot_not_uploadable", "Shoot is expired")
		}
		count, err := q.CountActivePhotosByShoot(ctx, store.CountActivePhotosByShootParams{PhotographerID: pgUUID(photographerID), ShootID: pgUUID(shootID)})
		if err != nil {
			return err
		}
		if int(count) >= s.maxPhotos {
			return errCode(409, "too_many_photos", "Too many photos for this shoot")
		}
		if _, err := q.ReservePhotoQuota(ctx, store.ReservePhotoQuotaParams{ID: pgUUID(photographerID), StorageUsedBytes: in.SizeBytes}); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return errCode(409, "quota_exceeded", "Storage quota exceeded")
			}
			return err
		}

		photoID = uuid.New()
		key = photostorage.OriginalKey(photographerID, shootID, photoID)
		previewKey := photostorage.PreviewKey(photographerID, shootID, photoID)
		thumbKey := photostorage.ThumbKey(photographerID, shootID, photoID)
		uid, err := s.objects.InitiateMultipart(ctx, key, in.ContentType)
		if err != nil {
			_ = q.ReleasePhotoQuota(ctx, store.ReleasePhotoQuotaParams{ID: pgUUID(photographerID), StorageUsedBytes: in.SizeBytes})
			return fmt.Errorf("initiate multipart: %w", err)
		}
		uploadID = uid
		if _, err := q.CreatePhoto(ctx, store.CreatePhotoParams{
			ID: pgUUID(photoID), PhotographerID: pgUUID(photographerID), ShootID: pgUUID(shootID), Filename: in.Filename, ContentType: in.ContentType, SizeBytes: in.SizeBytes,
			OriginalKey: key, PreviewKey: pgtype.Text{String: previewKey, Valid: true}, ThumbKey: pgtype.Text{String: thumbKey, Valid: true},
			Status: "uploading", SortKey: count, UploadID: pgtype.Text{String: uploadID, Valid: true},
		}); err != nil {
			_ = s.objects.AbortMultipart(ctx, key, uploadID)
			_ = q.ReleasePhotoQuota(ctx, store.ReleasePhotoQuotaParams{ID: pgUUID(photographerID), StorageUsedBytes: in.SizeBytes})
			return err
		}
		payload, _ := json.Marshal(map[string]any{"uploadId": uploadID, "partCount": partCount})
		if err := q.CreateShootEvent(ctx, store.CreateShootEventParams{
			PhotographerID: pgUUID(photographerID), ShootID: pgUUID(shootID), PhotoID: pgUUID(photoID), Kind: "photo_upload_created", Payload: payload,
		}); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return CreateUploadResult{}, err
	}

	n := int(partCount)
	if n > 3 {
		n = 3
	}
	parts := make([]UploadPartURL, 0, n)
	for i := 1; i <= n; i++ {
		u, err := s.objects.PresignUploadPart(ctx, key, uploadID, int32(i), photostorage.UploadPartTTL)
		if err != nil {
			return CreateUploadResult{}, err
		}
		parts = append(parts, UploadPartURL{PartNumber: int32(i), URL: u, ExpiresAt: time.Now().Add(photostorage.UploadPartTTL)})
	}
	return CreateUploadResult{PhotoID: photoID, ShootID: shootID, PartSizeBytes: partSizeBytes, PartCount: partCount, UploadParts: parts, Status: "uploading"}, nil
}

func (s *Service) PresignParts(ctx context.Context, photographerID, shootID, photoID uuid.UUID, partNumbers []int32) ([]UploadPartURL, error) {
	if len(partNumbers) == 0 || len(partNumbers) > maxPresignParts {
		return nil, errCode(400, "invalid_input", "partNumbers must contain 1-20 items")
	}
	var key, uploadID string
	var partCount int32
	err := database.WithTenant(ctx, s.pool, pgUUID(photographerID), func(tx pgx.Tx) error {
		q := store.New(tx)
		p, err := q.GetPhoto(ctx, store.GetPhotoParams{PhotographerID: pgUUID(photographerID), ShootID: pgUUID(shootID), ID: pgUUID(photoID)})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return errCode(404, "not_found", "Photo not found")
			}
			return err
		}
		if p.Status != "uploading" || !p.UploadID.Valid {
			return errCode(409, "invalid_photo_state", "Photo is not uploading")
		}
		key = p.OriginalKey
		uploadID = p.UploadID.String
		partCount = int32((p.SizeBytes + partSizeBytes - 1) / partSizeBytes)
		return nil
	})
	if err != nil {
		return nil, err
	}
	out := make([]UploadPartURL, 0, len(partNumbers))
	seen := map[int32]bool{}
	for _, pn := range partNumbers {
		if pn <= 0 || pn > partCount || seen[pn] {
			return nil, errCode(400, "invalid_input", "Invalid part number")
		}
		seen[pn] = true
		u, err := s.objects.PresignUploadPart(ctx, key, uploadID, pn, photostorage.UploadPartTTL)
		if err != nil {
			return nil, err
		}
		out = append(out, UploadPartURL{PartNumber: pn, URL: u, ExpiresAt: time.Now().Add(photostorage.UploadPartTTL)})
	}
	return out, nil
}

type CompletePart struct {
	PartNumber int32  `json:"partNumber"`
	ETag       string `json:"etag"`
}

func (s *Service) CompleteUpload(ctx context.Context, photographerID, shootID, photoID uuid.UUID, parts []CompletePart) (Photo, error) {
	if len(parts) == 0 {
		return Photo{}, errCode(400, "invalid_input", "parts required")
	}
	var key, uploadID string
	var declared int64
	err := database.WithTenant(ctx, s.pool, pgUUID(photographerID), func(tx pgx.Tx) error {
		q := store.New(tx)
		p, err := q.GetPhoto(ctx, store.GetPhotoParams{PhotographerID: pgUUID(photographerID), ShootID: pgUUID(shootID), ID: pgUUID(photoID)})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return errCode(404, "not_found", "Photo not found")
			}
			return err
		}
		if p.Status != "uploading" || !p.UploadID.Valid {
			return errCode(409, "invalid_photo_state", "Photo is not uploading")
		}
		key, uploadID, declared = p.OriginalKey, p.UploadID.String, p.SizeBytes
		return nil
	})
	if err != nil {
		return Photo{}, err
	}

	objParts := make([]photostorage.UploadedPart, 0, len(parts))
	for _, p := range parts {
		if p.PartNumber <= 0 || strings.TrimSpace(p.ETag) == "" {
			return Photo{}, errCode(400, "invalid_input", "invalid part")
		}
		objParts = append(objParts, photostorage.UploadedPart{PartNumber: p.PartNumber, ETag: p.ETag})
	}
	if err := s.objects.CompleteMultipart(ctx, key, uploadID, objParts); err != nil {
		return Photo{}, fmt.Errorf("complete multipart: %w", err)
	}
	info, err := s.objects.Head(ctx, key)
	if err != nil {
		return Photo{}, fmt.Errorf("head object: %w", err)
	}
	if info.Size != declared {
		_ = s.objects.Delete(ctx, key)
		_ = database.WithTenant(ctx, s.pool, pgUUID(photographerID), func(tx pgx.Tx) error {
			q := store.New(tx)
			if err := q.ReleasePhotoQuota(ctx, store.ReleasePhotoQuotaParams{ID: pgUUID(photographerID), StorageUsedBytes: declared}); err != nil {
				return err
			}
			_, err := q.MarkPhotoFailed(ctx, store.MarkPhotoFailedParams{PhotographerID: pgUUID(photographerID), ShootID: pgUUID(shootID), ID: pgUUID(photoID), Error: pgtype.Text{String: "size_mismatch", Valid: true}})
			return err
		})
		return Photo{}, errCode(422, "size_mismatch", "Uploaded size does not match declared size")
	}

	var out store.Photo
	err = database.WithTenant(ctx, s.pool, pgUUID(photographerID), func(tx pgx.Tx) error {
		q := store.New(tx)
		upd, err := q.MarkPhotoProcessing(ctx, store.MarkPhotoProcessingParams{PhotographerID: pgUUID(photographerID), ShootID: pgUUID(shootID), ID: pgUUID(photoID)})
		if err != nil {
			return err
		}
		out = store.Photo{ID: upd.ID, PhotographerID: upd.PhotographerID, ShootID: upd.ShootID, Filename: upd.Filename, ContentType: upd.ContentType, SizeBytes: upd.SizeBytes, Width: upd.Width, Height: upd.Height, OriginalKey: key, Status: upd.Status, Error: upd.Error, ProcessedAt: upd.ProcessedAt, SortKey: upd.SortKey, CreatedAt: upd.CreatedAt, UpdatedAt: upd.UpdatedAt, UploadID: upd.UploadID}
		if s.river != nil {
			_, err = s.river.InsertTx(ctx, tx, ProcessPhotoArgs{PhotographerID: photographerID, ShootID: shootID, PhotoID: photoID}, &river.InsertOpts{MaxAttempts: 5})
			if err != nil {
				return err
			}
		}
		payload, _ := json.Marshal(map[string]any{"size": declared})
		return q.CreateShootEvent(ctx, store.CreateShootEventParams{PhotographerID: pgUUID(photographerID), ShootID: pgUUID(shootID), PhotoID: pgUUID(photoID), Kind: "photo_upload_completed", Payload: payload})
	})
	if err != nil {
		return Photo{}, err
	}
	return toAPIPhoto(out, nil), nil
}

func (s *Service) List(ctx context.Context, photographerID, shootID uuid.UUID, limit, offset int32) (ListResult, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}
	var res ListResult
	err := database.WithTenant(ctx, s.pool, pgUUID(photographerID), func(tx pgx.Tx) error {
		q := store.New(tx)
		if _, err := q.GetShootForPhoto(ctx, store.GetShootForPhotoParams{PhotographerID: pgUUID(photographerID), ID: pgUUID(shootID)}); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return errCode(404, "not_found", "Shoot not found")
			}
			return err
		}
		rows, err := q.ListPhotosByShoot(ctx, store.ListPhotosByShootParams{PhotographerID: pgUUID(photographerID), ShootID: pgUUID(shootID), Limit: limit, Offset: offset})
		if err != nil {
			return err
		}
		tot, err := q.CountPhotosByShoot(ctx, store.CountPhotosByShootParams{PhotographerID: pgUUID(photographerID), ShootID: pgUUID(shootID)})
		if err != nil {
			return err
		}
		quota, err := q.GetPhotographerQuota(ctx, pgUUID(photographerID))
		if err != nil {
			return err
		}
		res.Totals.Count = tot.Total
		res.Totals.Bytes = tot.Bytes
		res.Quota.UsedBytes = quota.StorageUsedBytes
		res.Quota.QuotaBytes = quota.StorageQuotaBytes
		res.Items = make([]Photo, 0, len(rows))
		for _, r := range rows {
			var thumb *string
			if r.Status == "ready" && r.ThumbKey.Valid {
				u, err := s.objects.PresignGet(ctx, r.ThumbKey.String, photostorage.GetTTL, "")
				if err == nil {
					thumb = &u
				}
			}
			p := Photo{ID: uuid.UUID(r.ID.Bytes), ShootID: uuid.UUID(r.ShootID.Bytes), Filename: r.Filename, ContentType: r.ContentType, SizeBytes: r.SizeBytes, Status: r.Status, SortKey: r.SortKey, CreatedAt: r.CreatedAt.Time, UpdatedAt: r.UpdatedAt.Time, ThumbURL: thumb}
			if r.Width.Valid {
				v := r.Width.Int32
				p.Width = &v
			}
			if r.Height.Valid {
				v := r.Height.Int32
				p.Height = &v
			}
			if r.Error.Valid {
				v := r.Error.String
				p.Error = &v
			}
			if r.ProcessedAt.Valid {
				v := r.ProcessedAt.Time
				p.ProcessedAt = &v
			}
			res.Items = append(res.Items, p)
		}
		return nil
	})
	return res, err
}

func (s *Service) Delete(ctx context.Context, photographerID, shootID, photoID uuid.UUID) error {
	var orig, preview, thumb, uploadID, status string
	var size int64
	err := database.WithTenant(ctx, s.pool, pgUUID(photographerID), func(tx pgx.Tx) error {
		q := store.New(tx)
		row, err := q.DeletePhotoReturning(ctx, store.DeletePhotoReturningParams{PhotographerID: pgUUID(photographerID), ShootID: pgUUID(shootID), ID: pgUUID(photoID)})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return errCode(404, "not_found", "Photo not found")
			}
			return err
		}
		if err := q.ReleasePhotoQuota(ctx, store.ReleasePhotoQuotaParams{ID: pgUUID(photographerID), StorageUsedBytes: row.SizeBytes}); err != nil {
			return err
		}
		orig = row.OriginalKey
		if row.PreviewKey.Valid {
			preview = row.PreviewKey.String
		}
		if row.ThumbKey.Valid {
			thumb = row.ThumbKey.String
		}
		if row.UploadID.Valid {
			uploadID = row.UploadID.String
		}
		status = row.Status
		size = row.SizeBytes
		payload, _ := json.Marshal(map[string]any{"size": size, "photoId": photoID.String()})
		if err := q.CreateShootEvent(ctx, store.CreateShootEventParams{PhotographerID: pgUUID(photographerID), ShootID: pgUUID(shootID), PhotoID: pgtype.UUID{}, Kind: "photo_deleted", Payload: payload}); err != nil {
			return fmt.Errorf("create shoot event pid=%s: %w", photographerID.String(), err)
		}
		return nil
	})
	if err != nil {
		return err
	}
	if status == "uploading" && uploadID != "" {
		_ = s.objects.AbortMultipart(ctx, orig, uploadID)
	}
	for _, k := range []string{orig, preview, thumb} {
		if k == "" {
			continue
		}
		if err := s.objects.Delete(ctx, k); err != nil {
			s.logger.Warn("delete photo object failed", "key", k, "err", err)
		}
	}
	return nil
}

func (s *Service) Retry(ctx context.Context, photographerID, shootID, photoID uuid.UUID) (Photo, error) {
	var out store.Photo
	err := database.WithTenant(ctx, s.pool, pgUUID(photographerID), func(tx pgx.Tx) error {
		q := store.New(tx)
		p, err := q.GetPhoto(ctx, store.GetPhotoParams{PhotographerID: pgUUID(photographerID), ShootID: pgUUID(shootID), ID: pgUUID(photoID)})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return errCode(404, "not_found", "Photo not found")
			}
			return err
		}
		if p.Status != "failed" {
			return errCode(409, "invalid_photo_state", "Only failed photos can be retried")
		}
		upd, err := q.MarkPhotoProcessing(ctx, store.MarkPhotoProcessingParams{PhotographerID: pgUUID(photographerID), ShootID: pgUUID(shootID), ID: pgUUID(photoID)})
		if err != nil {
			return err
		}
		out = store.Photo{ID: upd.ID, PhotographerID: upd.PhotographerID, ShootID: upd.ShootID, Filename: upd.Filename, ContentType: upd.ContentType, SizeBytes: upd.SizeBytes, Width: upd.Width, Height: upd.Height, OriginalKey: p.OriginalKey, PreviewKey: p.PreviewKey, ThumbKey: p.ThumbKey, Status: upd.Status, Error: upd.Error, ProcessedAt: upd.ProcessedAt, SortKey: upd.SortKey, CreatedAt: upd.CreatedAt, UpdatedAt: upd.UpdatedAt, UploadID: upd.UploadID}
		if s.river != nil {
			if _, err := s.river.InsertTx(ctx, tx, ProcessPhotoArgs{PhotographerID: photographerID, ShootID: shootID, PhotoID: photoID}, &river.InsertOpts{MaxAttempts: 5}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return Photo{}, err
	}
	return toAPIPhoto(out, nil), nil
}

func toAPIPhoto(r store.Photo, thumb *string) Photo {
	p := Photo{ID: uuid.UUID(r.ID.Bytes), ShootID: uuid.UUID(r.ShootID.Bytes), Filename: r.Filename, ContentType: r.ContentType, SizeBytes: r.SizeBytes, Status: r.Status, SortKey: r.SortKey, CreatedAt: r.CreatedAt.Time, UpdatedAt: r.UpdatedAt.Time, ThumbURL: thumb}
	if r.Width.Valid {
		v := r.Width.Int32
		p.Width = &v
	}
	if r.Height.Valid {
		v := r.Height.Int32
		p.Height = &v
	}
	if r.Error.Valid {
		v := r.Error.String
		p.Error = &v
	}
	if r.ProcessedAt.Valid {
		v := r.ProcessedAt.Time
		p.ProcessedAt = &v
	}
	return p
}

func pgUUID(id uuid.UUID) pgtype.UUID { return pgtype.UUID{Bytes: id, Valid: true} }

func fromPGUUID(id pgtype.UUID) uuid.UUID { return uuid.UUID(id.Bytes) }
