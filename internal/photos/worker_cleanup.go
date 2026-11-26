package photos

import (
	"context"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"

	photostorage "github.com/fiveace-Merill/photobooth/internal/storage"
)

type DeleteObjectsArgs struct {
	Keys []string `json:"keys"`
}

func (DeleteObjectsArgs) Kind() string { return "delete_objects" }

type DeleteObjectsWorker struct {
	river.WorkerDefaults[DeleteObjectsArgs]
	objects photostorage.ObjectStore
	logger  *slog.Logger
}

func NewDeleteObjectsWorker(objects photostorage.ObjectStore, logger *slog.Logger) *DeleteObjectsWorker {
	if logger == nil {
		logger = slog.Default()
	}
	return &DeleteObjectsWorker{objects: objects, logger: logger}
}

func (w *DeleteObjectsWorker) Work(ctx context.Context, job *river.Job[DeleteObjectsArgs]) error {
	for _, k := range job.Args.Keys {
		if k == "" {
			continue
		}
		if err := w.objects.Delete(ctx, k); err != nil {
			w.logger.Warn("delete object failed", "key", k, "err", err)
			return err
		}
	}
	return nil
}

// EnqueueShootObjectCleanup is used by shoot deletion; it enqueues best-effort object cleanup.
func EnqueueShootObjectCleanup(ctx context.Context, riverClient *river.Client[pgx.Tx], photographerID uuid.UUID, shootID uuid.UUID, keys []string) error {
	if len(keys) == 0 || riverClient == nil {
		return nil
	}
	_, err := riverClient.Insert(ctx, DeleteObjectsArgs{Keys: keys}, nil)
	return err
}
