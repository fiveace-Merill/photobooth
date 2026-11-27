package photos

import (
	"bytes"
	"context"
	"log/slog"
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

type ProcessPhotoArgs struct {
	PhotographerID uuid.UUID `json:"photographerId"`
	ShootID        uuid.UUID `json:"shootId"`
	PhotoID        uuid.UUID `json:"photoId"`
}

func (ProcessPhotoArgs) Kind() string { return "process_photo" }

type ProcessPhotoWorker struct {
	river.WorkerDefaults[ProcessPhotoArgs]
	pool      *pgxpool.Pool
	objects   photostorage.ObjectStore
	maxMP     int
	logger    *slog.Logger
	testDelay time.Duration
}

func NewProcessPhotoWorker(pool *pgxpool.Pool, objects photostorage.ObjectStore, maxMP int, logger *slog.Logger) *ProcessPhotoWorker {
	if logger == nil {
		logger = slog.Default()
	}
	return &ProcessPhotoWorker{pool: pool, objects: objects, maxMP: maxMP, logger: logger}
}

func (w *ProcessPhotoWorker) WithTestDelay(d time.Duration) *ProcessPhotoWorker {
	if d > 0 {
		w.testDelay = d
	}
	return w
}

func (w *ProcessPhotoWorker) Work(ctx context.Context, job *river.Job[ProcessPhotoArgs]) error {
	args := job.Args

	var (
		origKey, previewKey, thumbKey string
		businessName                  string
		status                        string
	)
	err := database.WithTenant(ctx, w.pool, pgUUID(args.PhotographerID), func(tx pgx.Tx) error {
		q := store.New(tx)
		p, err := q.GetPhoto(ctx, store.GetPhotoParams{PhotographerID: pgUUID(args.PhotographerID), ShootID: pgUUID(args.ShootID), ID: pgUUID(args.PhotoID)})
		if err != nil {
			if err == pgx.ErrNoRows {
				return nil
			}
			return err
		}
		status = p.Status
		origKey = p.OriginalKey
		if p.PreviewKey.Valid {
			previewKey = p.PreviewKey.String
		}
		if p.ThumbKey.Valid {
			thumbKey = p.ThumbKey.String
		}
		_ = tx.QueryRow(ctx, `SELECT business_name FROM photographers WHERE id=$1`, pgUUID(args.PhotographerID)).Scan(&businessName)
		return nil
	})
	if err != nil {
		return err
	}
	if status == "" || status == "ready" {
		return nil
	}

	// Test-only delay for kill/recovery verification. Config ignores it in production.
	if w.testDelay > 0 {
		w.logger.Info("process_photo test delay", "photo_id", args.PhotoID, "delay", w.testDelay)
		t := time.NewTimer(w.testDelay)
		select {
		case <-ctx.Done():
			t.Stop()
			return ctx.Err()
		case <-t.C:
		}
	}

	rc, err := w.objects.Get(ctx, origKey)
	if err != nil {
		return err
	}
	processed, procErr := decodeAndProcess(ctx, rc, w.maxMP, businessName)
	_ = rc.Close()
	if procErr != nil {
		if isInvalidImage(procErr) {
			markErr := database.WithTenant(ctx, w.pool, pgUUID(args.PhotographerID), func(tx pgx.Tx) error {
				_, err := store.New(tx).MarkPhotoFailed(ctx, store.MarkPhotoFailedParams{PhotographerID: pgUUID(args.PhotographerID), ShootID: pgUUID(args.ShootID), ID: pgUUID(args.PhotoID), Error: pgtype.Text{String: "invalid_image", Valid: true}})
				return err
			})
			if markErr != nil {
				return markErr
			}
			return river.JobCancel(procErr)
		}
		return procErr
	}

	if previewKey != "" {
		if err := w.objects.Put(ctx, previewKey, "image/jpeg", bytes.NewReader(processed.Preview), int64(len(processed.Preview))); err != nil {
			return err
		}
	}
	if thumbKey != "" {
		if err := w.objects.Put(ctx, thumbKey, "image/jpeg", bytes.NewReader(processed.Thumb), int64(len(processed.Thumb))); err != nil {
			return err
		}
	}

	return database.WithTenant(ctx, w.pool, pgUUID(args.PhotographerID), func(tx pgx.Tx) error {
		_, err := store.New(tx).MarkPhotoReady(ctx, store.MarkPhotoReadyParams{
			PhotographerID: pgUUID(args.PhotographerID), ShootID: pgUUID(args.ShootID), ID: pgUUID(args.PhotoID),
			Width: pgtype.Int4{Int32: processed.Width, Valid: true}, Height: pgtype.Int4{Int32: processed.Height, Valid: true},
		})
		return err
	})
}
