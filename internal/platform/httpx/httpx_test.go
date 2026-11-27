package httpx

import (
	"net/http/httptest"
	"net/netip"
	"testing"
)

func mustPrefix(s string) netip.Prefix { return netip.MustParsePrefix(s) }

func TestClientIP(t *testing.T) {
	trusted := []netip.Prefix{mustPrefix("10.0.0.1/32"), mustPrefix("10.0.0.2/32")}
	tests := []struct {
		name    string
		remote  string
		xff     string
		trusted []netip.Prefix
		want    string
	}{
		{name: "untrusted ignores xff", remote: "203.0.113.9:1234", xff: "198.51.100.1", trusted: nil, want: "203.0.113.9"},
		{name: "trusted uses rightmost untrusted", remote: "10.0.0.1:1234", xff: "198.51.100.7, 10.0.0.2", trusted: trusted, want: "198.51.100.7"},
		{name: "several proxies", remote: "10.0.0.1:1234", xff: "203.0.113.5, 198.51.100.8, 10.0.0.2", trusted: trusted, want: "198.51.100.8"},
		{name: "ipv6", remote: "[2001:db8::1]:443", xff: "2001:db8::99, 2001:db8::2", trusted: []netip.Prefix{mustPrefix("2001:db8::1/128"), mustPrefix("2001:db8::2/128")}, want: "2001:db8::99"},
		{name: "malformed xff fallback remote", remote: "10.0.0.1:1234", xff: "garbage, 10.0.0.2", trusted: trusted, want: "10.0.0.1"},
		{name: "empty xff", remote: "10.0.0.1:1234", xff: "", trusted: trusted, want: "10.0.0.1"},
		{name: "all trusted returns remote", remote: "10.0.0.1:1234", xff: "10.0.0.2", trusted: trusted, want: "10.0.0.1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/", nil)
			r.RemoteAddr = tt.remote
			if tt.xff != "" {
				r.Header.Set("X-Forwarded-For", tt.xff)
			}
			if got := ClientIP(r, tt.trusted); got != tt.want {
				t.Fatalf("got %q want %q", got, tt.want)
			}
		})
	}
}
