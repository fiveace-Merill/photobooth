package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/fiveace-Merill/photobooth/internal/platform/httpx"
	"github.com/jackc/pgx/v5"
)

// Middleware loads the authenticated photographer from the session cookie.
// If a valid session exists, it stores the photographer in request context;
// otherwise the request continues without an authenticated photographer.
//
// Handlers that require auth should check PhotographerFromContext and return 401.
func NewMiddleware(svc *Service, cookieSecure bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c, err := r.Cookie(SessionCookieName)
			if err != nil || c.Value == "" {
				next.ServeHTTP(w, r)
				return
			}

			photog, err := svc.Authenticate(r.Context(), c.Value)
			if err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					// Invalid/expired cookie: clear it so the browser stops sending it.
					http.SetCookie(w, &http.Cookie{Name: SessionCookieName, Value: "", Path: "/", HttpOnly: true, Secure: cookieSecure, SameSite: http.SameSiteLaxMode, MaxAge: -1, Expires: time.Unix(0, 0)})
					next.ServeHTTP(w, r)
					return
				}
				// On unexpected DB errors, fail closed for safety.
				http.Error(w, "internal server error", http.StatusInternalServerError)
				return
			}

			ctx := context.WithValue(r.Context(), photographerContextKey, photog)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireAuth wraps a handler and rejects unauthenticated requests.
// Use it for endpoints that must have a logged-in photographer.
func RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := PhotographerFromContext(r.Context()); !ok {
			httpx.ErrorCode(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequireSameOriginJSON protects state-changing endpoints from cross-site requests.
// It requires a JSON content type and, when Origin is present, requires same origin.
// Same-origin browser requests usually omit Origin; cross-site POSTs include it.
func RequireSameOriginJSON(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}
		ct := strings.ToLower(r.Header.Get("Content-Type"))
		if !strings.HasPrefix(ct, "application/json") {
			httpx.ErrorCode(w, http.StatusUnsupportedMediaType, "invalid_content_type", "Content-Type must be application/json")
			return
		}
		origin := r.Header.Get("Origin")
		if origin != "" {
			expected := requestOrigin(r)
			if !sameOrigin(origin, expected) {
				httpx.ErrorCode(w, http.StatusForbidden, "invalid_origin", "invalid request origin")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func requestOrigin(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if forwarded := r.Header.Get("X-Forwarded-Proto"); forwarded != "" {
		scheme = strings.TrimSpace(strings.Split(forwarded, ",")[0])
	}
	host := r.Host
	if forwardedHost := r.Header.Get("X-Forwarded-Host"); forwardedHost != "" {
		host = strings.TrimSpace(strings.Split(forwardedHost, ",")[0])
	}
	return scheme + "://" + host
}

func sameOrigin(a, b string) bool {
	return strings.EqualFold(strings.TrimRight(a, "/"), strings.TrimRight(b, "/"))
}
