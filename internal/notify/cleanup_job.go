package notify

import (
	"context"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
)

type CleanupQueuedNotificationsArgs struct{}

func (CleanupQueuedNotificationsArgs) Kind() string { return "cleanup_queued_notifications" }

type CleanupQueuedNotificationsWorker struct {
	river.WorkerDefaults[CleanupQueuedNotificationsArgs]
	pool   *pgxpool.Pool
	logger *slog.Logger
}

func NewCleanupQueuedNotificationsWorker(pool *pgxpool.Pool, logger *slog.Logger) *CleanupQueuedNotificationsWorker {
	if logger == nil {
		logger = slog.Default()
	}
	return &CleanupQueuedNotificationsWorker{pool: pool, logger: logger}
}

func (w *CleanupQueuedNotificationsWorker) Work(ctx context.Context, _ *river.Job[CleanupQueuedNotificationsArgs]) error {
	res, err := w.pool.Exec(ctx, `UPDATE notifications SET status='failed', error_code='send_timeout', sent_at=NULL WHERE status='queued' AND created_at < now() - interval '10 minutes'`)
	if err != nil {
		return err
	}
	if n := res.RowsAffected(); n > 0 {
		w.logger.Warn("marked stale queued notifications failed", "count", n)
	}
	return nil
}
