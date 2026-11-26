package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/fiveace-Merill/photobooth/internal/jobs"
)

func main() {
	url := os.Getenv("MIGRATE_DATABASE_URL")
	if url == "" {
		fmt.Fprintln(os.Stderr, "MIGRATE_DATABASE_URL is required")
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer pool.Close()
	if err := jobs.RunMigrations(ctx, pool); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if _, err := pool.Exec(ctx, `GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO photobooth_app; GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO photobooth_app`); err != nil {
		fmt.Fprintln(os.Stderr, "river migrated but grant failed:", err)
		os.Exit(1)
	}
	fmt.Println("river migrate up ok")
}
