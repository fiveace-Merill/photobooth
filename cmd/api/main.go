// Command api is the HTTP server. It wires dependencies together and starts
// listening. Business logic does NOT belong here; it lives in internal/.
package main

import (
	"log/slog"
	"net/http"
	"os"
)

func main() {
	mux := http.NewServeMux()

	// Go 1.22+ lets you put the HTTP method in the pattern.
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		// Ignoring the write error is fine here: if the client has gone away
		// there is nothing useful we can do about it.
		_, _ = w.Write([]byte("ok"))
	})

	addr := ":8080"
	if port := os.Getenv("PORT"); port != "" {
		addr = ":" + port
	}

	slog.Info("api listening", "addr", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		slog.Error("server stopped", "err", err)
		os.Exit(1)
	}
}
