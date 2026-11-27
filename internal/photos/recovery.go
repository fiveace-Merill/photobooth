package photos

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/fiveace-Merill/photobooth/internal/photos/store"
	"github.com/fiveace-Merill/photobooth/internal/platform/database"
)

const (
	PhotoErrProcessingFailed   = "processing_failed"
	PhotoErrProcessingTimedOut = "processing_timeout"
)

// ProcessPhotoFinalizeHook marks photos failed when River gives up on a job.
// It runs in the job executor context; we must not return a DB-marking failure
// for non-retryable terminal states, otherwise the photo can remain "processing".
type ProcessPhotoFinalizeHook struct {
	river.HookDefaults
	pool *pgxpool.Pool
}

func NewProcessPhotoFinalizeHook(pool *pgxpool.Pool) *ProcessPhotoFinalizeHook {
	return &ProcessPhotoFinalizeHook{pool: pool}
}

func (h *ProcessPhotoFinalizeHook) WorkEnd(ctx context.Context, job *rivertype.JobRow, err error) error {
	if job == nil || job.Kind != (ProcessPhotoArgs{}).Kind() {
		return err
	}
	if err == nil {
		return nil
	}
	if job.State != rivertype.JobStateDiscarded && job.State != rivertype.JobStateCancelled {
		return err
	}
	code := PhotoErrProcessingFailed
	if job.State == rivertype.JobStateCancelled {
		code = PhotoErrProcessingTimedOut
	}
	var args ProcessPhotoArgs
	if uErr := jsonUnmarshalFn(job.EncodedArgs, &args); uErr != nil {
		return err
	}
	_ = markPhotoFailedIfNotTerminal(context.WithoutCancel(ctx), h.pool, args, code)
	return err
}

// MarkStaleProcessingPhotos fails photos stuck in processing with no active River
// job. It is a safety net for cases where River cannot surface a terminal hook.
// It uses a SECURITY DEFINER DB function because photos are RLS-protected and
// this system maintenance job must inspect across tenants.
func MarkStaleProcessingPhotos(ctx context.Context, pool *pgxpool.Pool) (int, error) {
	var n int
	if err := pool.QueryRow(ctx, `SELECT reap_stale_processing_photos()`).Scan(&n); err != nil {
		return 0, fmt.Errorf("reap stale processing photos: %w", err)
	}
	return n, nil
}

func markPhotoFailedIfNotTerminal(ctx context.Context, pool *pgxpool.Pool, args ProcessPhotoArgs, code string) error {
	return database.WithTenant(ctx, pool, pgUUID(args.PhotographerID), func(tx pgx.Tx) error {
		q := store.New(tx)
		p, err := q.GetPhoto(ctx, store.GetPhotoParams{PhotographerID: pgUUID(args.PhotographerID), ShootID: pgUUID(args.ShootID), ID: pgUUID(args.PhotoID)})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil
			}
			return err
		}
		if p.Status == "ready" || p.Status == "failed" {
			return nil
		}
		_, err = q.MarkPhotoFailed(ctx, store.MarkPhotoFailedParams{
			PhotographerID: pgUUID(args.PhotographerID), ShootID: pgUUID(args.ShootID), ID: pgUUID(args.PhotoID),
			Error: pgtype.Text{String: code, Valid: true},
		})
		return err
	})
}

func uuidFromPg(id pgtype.UUID) uuid.UUID { return uuid.UUID(id.Bytes) }

// jsonUnmarshalFn is defined in json.go.
