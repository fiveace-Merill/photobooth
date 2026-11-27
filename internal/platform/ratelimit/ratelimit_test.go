package ratelimit

import (
	"net/http/httptest"
	"testing"
	"time"
)

func TestLimiter(t *testing.T) {
	l := New(2, time.Minute)
	if !l.Allow("1.2.3.4") || !l.Allow("1.2.3.4") {
		t.Fatal("expected first two allowed")
	}
	if l.Allow("1.2.3.4") {
		t.Fatal("expected third denied")
	}
	if !l.Allow("5.6.7.8") {
		t.Fatal("different IP should be allowed")
	}
}

func TestClientIP(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "10.0.0.1:1234"
	if got := ClientIP(r, nil); got != "10.0.0.1" {
		t.Fatalf("got %q", got)
	}
	r.Header.Set("X-Forwarded-For", "1.2.3.4, 10.0.0.1")
	if got := ClientIP(r, nil); got != "10.0.0.1" {
		t.Fatalf("untrusted proxy should ignore XFF, got %q", got)
	}
}
