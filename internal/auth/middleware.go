package auth

import (
	"context"
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"
)

// Middleware loads the authenticated photographer from the session cookie.
// If a valid session exists, it stores the photographer in request context;
// otherwise the request continues without an authenticated photographer.
//
// Handlers that require auth should check PhotographerFromContext and return 401.
func NewMiddleware(svc *Service) func(http.Handler) http.Handler {
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
					http.SetCookie(w, &http.Cookie{Name: SessionCookieName, Value: "", Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode, MaxAge: -1})
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
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
