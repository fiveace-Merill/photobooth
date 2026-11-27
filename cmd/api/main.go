// Command api is the HTTP server. It wires dependencies together and starts
// listening. Business logic does NOT belong here; it lives in internal/.
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/riverqueue/river"

	"github.com/fiveace-Merill/photobooth/internal/auth"
	authstore "github.com/fiveace-Merill/photobooth/internal/auth/store"
	"github.com/fiveace-Merill/photobooth/internal/clients"
	"github.com/fiveace-Merill/photobooth/internal/deliveries"
	"github.com/fiveace-Merill/photobooth/internal/jobs"
	"github.com/fiveace-Merill/photobooth/internal/notify"
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
	photosWorker := photos.NewProcessPhotoWorker(pool, store, cfg.MaxImageMegapixels, slog.Default()).WithTestDelay(cfg.PhotoTestDelay)
	river.AddWorker(riverWorkers, photosWorker)
	river.AddWorker(riverWorkers, photos.NewDeleteObjectsWorker(store, slog.Default()))
	river.AddWorker(riverWorkers, photos.NewReapStaleProcessingWorker(pool, slog.Default()))
	river.AddWorker(riverWorkers, notify.NewCleanupQueuedNotificationsWorker(pool, slog.Default()))
	riverClient, err := jobs.NewClient(pool, riverWorkers, cfg.RiverMaxWorkers, slog.Default(), photos.NewProcessPhotoFinalizeHook(pool))
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
	notifySvc, err := notify.NewServiceFromConfig(pool, notify.FactoryConfig{
		AppEnv:                         strings.ToLower(strings.TrimSpace(os.Getenv("APP_ENV"))),
		EmailProvider:                  cfg.EmailProvider,
		SMTPHost:                       cfg.SMTPHost,
		SMTPPort:                       cfg.SMTPPort,
		SMTPUser:                       cfg.SMTPUser,
		SMTPPassword:                   cfg.SMTPPassword,
		SMTPTLS:                        cfg.SMTPTLS,
		MailFromAddress:                cfg.MailFromAddress,
		MailFromName:                   cfg.MailFromName,
		EmailDailyLimitPerPhotographer: cfg.EmailDailyLimitPerPhotographer,
	}, slog.Default())
	if err != nil {
		slog.Error("init notify service", "err", err)
		os.Exit(1)
	}
	deliveriesSvc := deliveries.NewService(pool, store, cfg.PublicAppURL, deliveries.NewNotifyAdapter(notifySvc))
	deliveriesHandler := deliveries.NewHandler(deliveriesSvc)

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
	deliveriesHandler.Register(mux, func(h http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) { auth.RequireAuth(http.HandlerFunc(h)).ServeHTTP(w, r) }
	})

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           authMiddleware(mux),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		// WriteTimeout is intentionally unset so unlocked delivery zip streams
		// can run longer than normal API requests. Public handlers set
		// Cache-Control/Referrer-Policy and download handler sets a per-request
		// write deadline.
		IdleTimeout: 120 * time.Second,
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := riverClient.Start(ctx); err != nil {
		slog.Error("start river", "err", err)
		os.Exit(1)
	}
	// Safety net: if River discards/cancels a job and the finalize hook cannot
	// update the photo, reap stale processing rows periodically.
	riverClient.PeriodicJobs().Add(river.NewPeriodicJob(
		river.PeriodicInterval(5*time.Minute),
		func() (river.JobArgs, *river.InsertOpts) { return photos.ReapStaleProcessingArgs{}, nil },
		&river.PeriodicJobOpts{RunOnStart: true},
	))
	riverClient.PeriodicJobs().Add(river.NewPeriodicJob(
		river.PeriodicInterval(5*time.Minute),
		func() (river.JobArgs, *river.InsertOpts) { return notify.CleanupQueuedNotificationsArgs{}, nil },
		&river.PeriodicJobOpts{RunOnStart: true},
	))
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
