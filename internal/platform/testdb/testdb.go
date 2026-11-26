package testdb

import (
	"context"
	"crypto/rand"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/fiveace-Merill/photobooth/internal/jobs"
)

// DB holds connections for integration tests against a real Postgres database.
// Owner is used for setup/admin tasks; App simulates the API's non-owner role,
// which is subject to row-level security.
type DB struct {
	Name     string
	Owner    *pgxpool.Pool
	App      *pgxpool.Pool
	ownerURL string
	appURL   string
}

// Open creates a fresh temporary database, applies all goose SQL migrations,
// ensures the photobooth_app role can access the schema, and returns owner/app pools.
//
// Tests use this instead of mocks because tenant isolation and SQL behavior are
// security-sensitive and easy to get wrong with fake databases.
func Open(t *testing.T) *DB {
	t.Helper()

	baseURL := os.Getenv("MIGRATE_DATABASE_URL")
	if baseURL == "" {
		t.Skip("MIGRATE_DATABASE_URL not set; skipping integration test")
	}
	appBaseURL := os.Getenv("DATABASE_URL")
	if appBaseURL == "" {
		t.Skip("DATABASE_URL not set; skipping integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	dbName := "photobooth_test_" + randomSuffix()
	ownerURL := replaceDBName(t, baseURL, dbName)
	appURL := replaceDBName(t, appBaseURL, dbName)

	adminPool := connect(t, ctx, replaceDBName(t, baseURL, "postgres"))
	_, err := adminPool.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{dbName}.Sanitize())
	if err != nil {
		adminPool.Close()
		t.Fatalf("create test database: %v", err)
	}
	adminPool.Close()

	ownerPool := connect(t, ctx, ownerURL)
	tdb := &DB{Name: dbName, Owner: ownerPool, ownerURL: ownerURL, appURL: appURL}

	// Cleanup runs even when the test fails. We close pools first, then drop DB.
	t.Cleanup(func() {
		tdb.Close(context.Background())
		dropCtx, dropCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer dropCancel()
		admin := connectCleanup(t, dropCtx, replaceDBName(t, baseURL, "postgres"))
		if admin == nil {
			return
		}
		defer admin.Close()
		_, _ = admin.Exec(dropCtx, "DROP DATABASE IF EXISTS "+pgx.Identifier{dbName}.Sanitize()+" WITH (FORCE)")
	})

	if err := applyMigrations(ctx, ownerPool, migrationsDir(t)); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}
	if err := jobs.RunMigrations(ctx, ownerPool); err != nil {
		t.Fatalf("apply river migrations: %v", err)
	}
	if err := ensureAppRole(ctx, ownerPool); err != nil {
		t.Fatalf("ensure app role grants: %v", err)
	}

	tdb.App = connect(t, ctx, appURL)
	return tdb
}

// Close closes the owner and app pools.
func (db *DB) Close(ctx context.Context) {
	if db.App != nil {
		db.App.Close()
	}
	if db.Owner != nil {
		db.Owner.Close()
	}
}

func connect(t *testing.T, ctx context.Context, databaseURL string) *pgxpool.Pool {
	t.Helper()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect %s: %v", sanitizeURL(databaseURL), err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Fatalf("ping %s: %v", sanitizeURL(databaseURL), err)
	}
	return pool
}

func connectCleanup(t *testing.T, ctx context.Context, databaseURL string) *pgxpool.Pool {
	t.Helper()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Logf("cleanup connect %s: %v", sanitizeURL(databaseURL), err)
		return nil
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Logf("cleanup ping %s: %v", sanitizeURL(databaseURL), err)
		return nil
	}
	return pool
}

func randomSuffix() string {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand failure is extremely rare; fall back to time for uniqueness.
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("%x", b[:])
}

func replaceDBName(t *testing.T, rawURL, dbName string) string {
	t.Helper()
	u, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}
	u.Path = "/" + dbName
	return u.String()
}

func sanitizeURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "<invalid-url>"
	}
	if u.User != nil {
		u.User = url.UserPassword(u.User.Username(), "***")
	}
	return u.String()
}

func migrationsDir(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// this file: <root>/internal/platform/testdb/testdb.go
	root := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", "..", ".."))
	dir := filepath.Join(root, "migrations")
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("migrations dir: %v", err)
	}
	return dir
}

var migrationFileRE = regexp.MustCompile(`^\d+_.+\.sql$`)

func applyMigrations(ctx context.Context, pool *pgxpool.Pool, dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() || !migrationFileRE.MatchString(e.Name()) {
			continue
		}
		content, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return fmt.Errorf("read %s: %w", e.Name(), err)
		}
		up := extractGooseUp(string(content))
		if strings.TrimSpace(up) == "" {
			continue
		}
		// Execute the migration as a single script. pgx Exec supports multi-statement
		// simple query protocol in many cases; for safety and clarity we use the
		// pgx simple protocol via Exec with the full script.
		if _, err := pool.Exec(ctx, up); err != nil {
			return fmt.Errorf("apply %s: %w", e.Name(), err)
		}
	}
	return nil
}

// extractGooseUp returns the SQL between "-- +goose Up" and "-- +goose Down".
// We keep this parser small and specific because migration files in this repo
// follow a simple goose format.
func extractGooseUp(content string) string {
	lines := strings.Split(content, "\n")
	var up []string
	inUp := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "-- +goose Up" {
			inUp = true
			continue
		}
		if strings.HasPrefix(trimmed, "-- +goose Down") {
			break
		}
		if inUp {
			up = append(up, line)
		}
	}
	return strings.Join(up, "\n")
}

// ensureAppRole makes sure the photobooth_app role exists in the test Postgres
// cluster and has grants on the newly created test database/schema.
// db/init/01_roles.sh only runs on a fresh Docker volume; test DBs are created
// dynamically, so tests must grant access explicitly.
func ensureAppRole(ctx context.Context, pool *pgxpool.Pool) error {
	// Create role if missing. We give it NOBYPASSRLS so RLS is actually enforced.
	// Password is irrelevant for object grants, but role may be used for login by
	// DATABASE_URL in local dev; we keep creation minimal and idempotent.
	_, err := pool.Exec(ctx, `
DO $$
BEGIN
	IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'photobooth_app') THEN
		CREATE ROLE photobooth_app LOGIN PASSWORD 'test-app' NOSUPERUSER NOBYPASSRLS;
	END IF;
END $$`)
	if err != nil {
		return fmt.Errorf("create role: %w", err)
	}

	stmts := []string{
		"GRANT CONNECT ON DATABASE " + pgx.Identifier{currentDatabase(ctx, pool)}.Sanitize() + " TO photobooth_app",
		"GRANT USAGE ON SCHEMA public TO photobooth_app",
		"GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO photobooth_app",
		"GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO photobooth_app",
		"GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA public TO photobooth_app",
		"ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO photobooth_app",
		"ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT USAGE, SELECT ON SEQUENCES TO photobooth_app",
		"ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT EXECUTE ON FUNCTIONS TO photobooth_app",
	}
	for _, stmt := range stmts {
		if _, err := pool.Exec(ctx, stmt); err != nil {
			return fmt.Errorf("%s: %w", stmt, err)
		}
	}
	return nil
}

func currentDatabase(ctx context.Context, pool *pgxpool.Pool) string {
	var name string
	if err := pool.QueryRow(ctx, "SELECT current_database()").Scan(&name); err != nil {
		return ""
	}
	return name
}
