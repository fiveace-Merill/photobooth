package admin

import (
	"net/http"
	"strings"

	"github.com/fiveace-Merill/photobooth/internal/platform/httpx"
)

func RequireStateChangingOrigin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}
		ct := r.Header.Get("Content-Type")
		if !strings.HasPrefix(ct, "application/json") {
			httpx.ErrorCode(w, http.StatusUnsupportedMediaType, "invalid_content_type", "Content-Type must be application/json")
			return
		}
		origin := strings.TrimSpace(r.Header.Get("Origin"))
		if origin == "" {
			httpx.ErrorCode(w, http.StatusForbidden, "invalid_origin", "Origin required")
			return
		}
		scheme := "http"
		if r.TLS != nil {
			scheme = "https"
		}
		if fwd := r.Header.Get("X-Forwarded-Proto"); fwd != "" {
			scheme = strings.TrimSpace(strings.Split(fwd, ",")[0])
		}
		if origin != scheme+"://"+r.Host {
			httpx.ErrorCode(w, http.StatusForbidden, "invalid_origin", "Invalid origin")
			return
		}
		next.ServeHTTP(w, r)
	})
}
