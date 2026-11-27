package admin

import (
	"context"
	"net/http"
	"net/netip"
	"strings"

	"github.com/fiveace-Merill/photobooth/internal/platform/httpx"
)

const SessionCookieName = "pb_admin"

type Middleware struct {
	svc            *Service
	cookieSecure   bool
	allowedIPs     []netip.Prefix
	trustedProxies []netip.Prefix
}

func NewMiddleware(svc *Service, cookieSecure bool, allowedIPs, trustedProxies []netip.Prefix) *Middleware {
	return &Middleware{svc: svc, cookieSecure: cookieSecure, allowedIPs: append([]netip.Prefix(nil), allowedIPs...), trustedProxies: append([]netip.Prefix(nil), trustedProxies...)}
}

func (m *Middleware) ipAllowed(r *http.Request) bool {
	if len(m.allowedIPs) == 0 {
		return true
	}
	host := httpx.ClientIP(r, m.trustedProxies)
	addr, err := netip.ParseAddr(host)
	if err != nil {
		return false
	}
	addr = addr.Unmap()
	for _, p := range m.allowedIPs {
		if p.Contains(addr) {
			return true
		}
	}
	return false
}

func (m *Middleware) Load(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !m.ipAllowed(r) {
			http.NotFound(w, r)
			return
		}
		c, err := r.Cookie(SessionCookieName)
		if err != nil || strings.TrimSpace(c.Value) == "" {
			next.ServeHTTP(w, r)
			return
		}
		a, hash, pending, err := m.svc.Authenticate(r.Context(), c.Value)
		if err != nil {
			clearAdminCookie(w, m.cookieSecure)
			next.ServeHTTP(w, r)
			return
		}
		ctx := WithAdmin(r.Context(), a)
		ctx = WithSessionTokenHash(ctx, hash)
		ctx = contextWithPending(ctx, pending)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

type pendingKey string

const pendingContextKey pendingKey = "pending"

func contextWithPending(ctx context.Context, pending bool) context.Context {
	return context.WithValue(ctx, pendingContextKey, pending)
}

func PendingFromContext(ctx context.Context) bool {
	v, _ := ctx.Value(pendingContextKey).(bool)
	return v
}

func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := AdminFromContext(r.Context()); !ok {
			httpx.ErrorCode(w, http.StatusUnauthorized, "unauthorized", "authentication required")
			return
		}
		if PendingFromContext(r.Context()) {
			httpx.ErrorCode(w, http.StatusUnauthorized, "unauthorized", "authentication required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func RequirePendingAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := AdminFromContext(r.Context()); !ok || !PendingFromContext(r.Context()) {
			httpx.ErrorCode(w, http.StatusUnauthorized, "unauthorized", "authentication required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func clearAdminCookie(w http.ResponseWriter, secure bool) {
	http.SetCookie(w, &http.Cookie{Name: SessionCookieName, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: secure, SameSite: http.SameSiteStrictMode})
}

func SetAdminCookie(w http.ResponseWriter, raw string, secure bool, maxAge int) {
	http.SetCookie(w, &http.Cookie{Name: SessionCookieName, Value: raw, Path: "/", MaxAge: maxAge, HttpOnly: true, Secure: secure, SameSite: http.SameSiteStrictMode})
}

func RequireSameOriginJSON(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ct := r.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
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
		expected := scheme + "://" + r.Host
		if origin != expected {
			httpx.ErrorCode(w, http.StatusForbidden, "invalid_origin", "Invalid origin")
			return
		}
		next.ServeHTTP(w, r)
	})
}
