// Command api is the HTTP server. It wires dependencies together and starts
// listening. Business logic does NOT belong here; it lives in internal/.
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/fiveace-Merill/photobooth/internal/auth"
	authstore "github.com/fiveace-Merill/photobooth/internal/auth/store"
	"github.com/fiveace-Merill/photobooth/internal/platform/config"
	"github.com/fiveace-Merill/photobooth/internal/platform/database"
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
	cancel()
	if err != nil {
		slog.Error("open database", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	authSvc := auth.NewService(authstore.New(pool))
	authHandler := auth.NewHandler(authSvc, cfg.CookieSecure)
	authMiddleware := auth.NewMiddleware(authSvc)

	mux := http.NewServeMux()

	// Go 1.22+ lets you put the HTTP method in the pattern.
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		// Ignoring the write error is fine here: if the client has gone away
		// there is nothing useful we can do about it.
		_, _ = w.Write([]byte("ok"))
	})

	authHandler.Register(mux, authMiddleware)

	// Wrap the whole mux with auth middleware so authenticated requests get the
	// photographer loaded into context. Individual handlers can require auth.
	handler := authMiddleware(mux)

	addr := ":" + cfg.Port
	slog.Info("api listening", "addr", addr)
	if err := http.ListenAndServe(addr, handler); err != nil {
		slog.Error("server stopped", "err", err)
		os.Exit(1)
	}
}
