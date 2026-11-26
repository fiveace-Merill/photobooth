// Command api is the HTTP server. It wires dependencies together and starts
// listening. Business logic does NOT belong here; it lives in internal/.
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/riverqueue/river"

	"github.com/fiveace-Merill/photobooth/internal/auth"
	authstore "github.com/fiveace-Merill/photobooth/internal/auth/store"
	"github.com/fiveace-Merill/photobooth/internal/clients"
	"github.com/fiveace-Merill/photobooth/internal/jobs"
	"github.com/fiveace-Merill/photobooth/internal/photos"
	"github.com/fiveace-Merill/photobooth/internal/platform/config"
	"github.com/fiveace-Merill/photobooth/internal/platform/database"
	"github.com/fiveace-Merill/photobooth/internal/shoots"
	photostorage "github.com/fiveace-Merill/photobooth/internal/storage"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("load config", "err", err)
		os.Exit(1)
	}

	// Use a startup timeout so the process doesn't hang forever if DB is down.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	pool, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		cancel()
		slog.Error("open database", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	// River migrations require table-creation privileges. Prefer owner URL when present;
	// otherwise attempt with app pool (works in tests/local owner setups).
	migrateURL := os.Getenv("MIGRATE_DATABASE_URL")
	if migrateURL != "" {
		ownerPool, err := database.Open(ctx, migrateURL)
		if err != nil {
			cancel()
			slog.Error("open migrate database", "err", err)
			os.Exit(1)
		}
		err = jobs.RunMigrations(ctx, ownerPool)
		ownerPool.Close()
		if err != nil {
			cancel()
			slog.Error("run river migrations", "err", err)
			os.Exit(1)
		}
	} else {
		if err := jobs.RunMigrations(ctx, pool); err != nil {
			cancel()
			slog.Error("run river migrations (set MIGRATE_DATABASE_URL for owner privileges)", "err", err)
			os.Exit(1)
		}
	}
	cancel()

	var store photostorage.ObjectStore
	switch cfg.StorageDriver {
	case "fake":
		store = photostorage.NewFake()
	case "r2":
		r2, err := photostorage.NewR2(context.Background(), photostorage.R2Config{
			AccountID: cfg.R2AccountID, AccessKeyID: cfg.R2AccessKeyID, SecretAccessKey: cfg.R2SecretAccessKey, Bucket: cfg.R2Bucket,
		})
		if err != nil {
			slog.Error("init r2 storage", "err", err)
			os.Exit(1)
		}
		store = r2
	default:
		slog.Error("unsupported storage driver", "driver", cfg.StorageDriver)
		os.Exit(1)
	}

	riverWorkers := river.NewWorkers()
	river.AddWorker(riverWorkers, photos.NewProcessPhotoWorker(pool, store, cfg.MaxImageMegapixels, slog.Default()))
	river.AddWorker(riverWorkers, photos.NewDeleteObjectsWorker(store, slog.Default()))
	riverClient, err := jobs.NewClient(pool, riverWorkers, cfg.RiverMaxWorkers, slog.Default())
	if err != nil {
		slog.Error("init river client", "err", err)
		os.Exit(1)
	}

	authSvc := auth.NewService(authstore.New(pool), cfg.SignupEnabled)
	authHandler := auth.NewHandler(authSvc, cfg.CookieSecure)
	authMiddleware := auth.NewMiddleware(authSvc, cfg.CookieSecure)

	clientsSvc := clients.NewService(pool)
	clientsHandler := clients.NewHandler(clientsSvc)
	shootsSvc, err := shoots.NewService(pool, clientsSvc)
	if err != nil {
		slog.Error("init shoots service", "err", err)
		os.Exit(1)
	}
	shootsSvc.WithRiver(riverClient)
	shootsHandler := shoots.NewHandler(shootsSvc)
	photosSvc := photos.NewService(photos.Deps{Pool: pool, Objects: store, River: riverClient, MaxPhotoBytes: cfg.MaxPhotoBytes, MaxPhotosPerShoot: cfg.MaxPhotosPerShoot, MaxImageMegapixels: cfg.MaxImageMegapixels, Logger: slog.Default()})
	photosHandler := photos.NewHandler(photosSvc)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	authHandler.Register(mux, authMiddleware)
	clientsHandler.Register(mux, auth.RequireAuth)
	shootsHandler.Register(mux, auth.RequireAuth)
	photosHandler.Register(mux, func(h http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) { auth.RequireAuth(http.HandlerFunc(h)).ServeHTTP(w, r) }
	})

	srv := &http.Server{Addr: ":" + cfg.Port, Handler: authMiddleware(mux)}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := riverClient.Start(ctx); err != nil {
		slog.Error("start river", "err", err)
		os.Exit(1)
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer stopCancel()
		_ = riverClient.Stop(stopCtx)
	}()

	slog.Info("api listening", "addr", srv.Addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("server stopped", "err", err)
		os.Exit(1)
	}
}
