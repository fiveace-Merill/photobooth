package jobs

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"
	"github.com/riverqueue/river/rivertype"
)

// RunMigrations applies River schema migrations. Use owner-capable DB in prod.
func RunMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	migrator, err := rivermigrate.New(riverpgxv5.New(pool), nil)
	if err != nil {
		return fmt.Errorf("river migrator: %w", err)
	}
	_, err = migrator.Migrate(ctx, rivermigrate.DirectionUp, nil)
	if err != nil {
		return fmt.Errorf("river migrate up: %w", err)
	}
	return nil
}

const (
	// ProcessPhotoJobTimeout bounds a single image decode/resize/watermark/upload.
	// River's default is 1 minute, which is tight for large photos on small instances.
	ProcessPhotoJobTimeout = 3 * time.Minute
	// RescueStuckJobsAfter determines when River's rescuer requeues jobs left in
	// "running" after a crash/kill -9. River default is 1 hour, which left slice 3a
	// kill-test jobs stuck. It must be greater than JobTimeout or River rejects config.
	ProcessPhotoRescueStuckJobsAfter = 5 * time.Minute
)

// NewClient builds a River client. Workers must be registered before Start.
func NewClient(pool *pgxpool.Pool, workers *river.Workers, maxWorkers int, logger *slog.Logger, hooks ...rivertype.Hook) (*river.Client[pgx.Tx], error) {
	if maxWorkers <= 0 {
		maxWorkers = 1
	}
	if ProcessPhotoRescueStuckJobsAfter <= ProcessPhotoJobTimeout {
		return nil, fmt.Errorf("RescueStuckJobsAfter must be greater than JobTimeout")
	}
	cfg := &river.Config{
		Queues: map[string]river.QueueConfig{
			river.QueueDefault: {MaxWorkers: maxWorkers},
		},
		Workers: workers,
		// Recovery: River defaults are JobTimeout=1m and RescueStuckJobsAfter=1h.
		// A kill -9 leaves jobs running until rescued; 3m/5m makes crash recovery
		// observable and bounded while giving image processing enough time.
		JobTimeout:           ProcessPhotoJobTimeout,
		RescueStuckJobsAfter: ProcessPhotoRescueStuckJobsAfter,
		FetchCooldown:        100 * time.Millisecond,
		FetchPollInterval:    1 * time.Second,
		Logger:               logger,
		Hooks:                hooks,
	}
	client, err := river.NewClient(riverpgxv5.New(pool), cfg)
	if err != nil {
		return nil, fmt.Errorf("river client: %w", err)
	}
	return client, nil
}
