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

// NewClient builds a River client. Workers must be registered before Start.
func NewClient(pool *pgxpool.Pool, workers *river.Workers, maxWorkers int, logger *slog.Logger) (*river.Client[pgx.Tx], error) {
	if maxWorkers <= 0 {
		maxWorkers = 2
	}
	cfg := &river.Config{
		Queues: map[string]river.QueueConfig{
			river.QueueDefault: {MaxWorkers: maxWorkers},
		},
		Workers:           workers,
		FetchCooldown:     100 * time.Millisecond,
		FetchPollInterval: 1 * time.Second,
		Logger:            logger,
	}
	client, err := river.NewClient(riverpgxv5.New(pool), cfg)
	if err != nil {
		return nil, fmt.Errorf("river client: %w", err)
	}
	return client, nil
}
