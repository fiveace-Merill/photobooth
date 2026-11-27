package ratelimit

import (
	"net/http"
	"net/netip"
	"sync"
	"time"

	"github.com/fiveace-Merill/photobooth/internal/platform/httpx"
)

// Limiter is a simple per-process, in-memory IP rate limiter.
// It resets on restart; this is acceptable for abuse protection on public endpoints.
type Limiter struct {
	mu     sync.Mutex
	hits   map[string][]time.Time
	limit  int
	window time.Duration
}

func New(limit int, window time.Duration) *Limiter {
	return &Limiter{hits: map[string][]time.Time{}, limit: limit, window: window}
}

func (l *Limiter) Allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	cutoff := now.Add(-l.window)
	hits := l.hits[ip][:0]
	for _, t := range l.hits[ip] {
		if t.After(cutoff) {
			hits = append(hits, t)
		}
	}
	if len(hits) >= l.limit {
		l.hits[ip] = hits
		return false
	}
	l.hits[ip] = append(hits, now)
	return true
}

func ClientIP(r *http.Request, trusted []netip.Prefix) string {
	return httpx.ClientIP(r, trusted)
}
