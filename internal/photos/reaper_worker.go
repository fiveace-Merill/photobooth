package photos

import (
	"context"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
)

type ReapStaleProcessingArgs struct{}

func (ReapStaleProcessingArgs) Kind() string { return "reap_stale_processing_photos" }

type ReapStaleProcessingWorker struct {
	river.WorkerDefaults[ReapStaleProcessingArgs]
	pool   *pgxpool.Pool
	logger *slog.Logger
}

func NewReapStaleProcessingWorker(pool *pgxpool.Pool, logger *slog.Logger) *ReapStaleProcessingWorker {
	if logger == nil {
		logger = slog.Default()
	}
	return &ReapStaleProcessingWorker{pool: pool, logger: logger}
}

func (w *ReapStaleProcessingWorker) Work(ctx context.Context, _ *river.Job[ReapStaleProcessingArgs]) error {
	n, err := MarkStaleProcessingPhotos(ctx, w.pool)
	if err != nil {
		return err
	}
	if n > 0 {
		w.logger.Warn("reaped stale processing photos", "count", n)
	}
	return nil
}
