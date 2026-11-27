package httpx

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"strings"
)

// ErrorResponse is the consistent JSON shape returned by API errors.
// Keeping error responses uniform makes frontend handling predictable.
// Code is a stable machine-readable identifier; Error is human-readable.
type ErrorResponse struct {
	Error string `json:"error"`
	Code  string `json:"code,omitempty"`
}

// WriteJSON writes a JSON response with the given status code.
// The Content-Type is set before WriteHeader because headers cannot be changed
// after the status line has been written.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		// At this point headers/status are already sent; logging is the best we can do.
		slog.Error("write json response", "err", err)
	}
}

// Error writes a JSON error response. Internal details should not be exposed to
// clients; pass a safe, user-meaningful message.
func Error(w http.ResponseWriter, status int, message string) {
	WriteJSON(w, status, ErrorResponse{Error: message})
}

// ErrorCode writes a JSON error response with a stable machine-readable code.
func ErrorCode(w http.ResponseWriter, status int, code, message string) {
	WriteJSON(w, status, ErrorResponse{Error: message, Code: code})
}

// ClientIP returns the client IP for request policy decisions.
// If the direct peer is not trusted, X-Forwarded-For is ignored completely.
// If trusted, it walks XFF right-to-left and returns the rightmost entry that
// is not itself a trusted proxy. Malformed/empty input falls back to RemoteAddr.
func ClientIP(r *http.Request, trusted []netip.Prefix) string {
	remote := remoteAddrIP(r.RemoteAddr)
	if remote == "" {
		return ""
	}
	remoteAddr, err := netip.ParseAddr(remote)
	if err != nil {
		return remote
	}
	remoteAddr = remoteAddr.Unmap()
	if !ipInPrefixes(remoteAddr, trusted) {
		return remoteAddr.String()
	}

	xff := r.Header.Get("X-Forwarded-For")
	if strings.TrimSpace(xff) == "" {
		return remoteAddr.String()
	}
	parts := strings.Split(xff, ",")
	for i := len(parts) - 1; i >= 0; i-- {
		ipStr := strings.TrimSpace(parts[i])
		if ipStr == "" {
			continue
		}
		if h, _, err := net.SplitHostPort(ipStr); err == nil {
			ipStr = h
		}
		ipStr = strings.Trim(ipStr, "[]")
		addr, err := netip.ParseAddr(ipStr)
		if err != nil {
			return remoteAddr.String()
		}
		addr = addr.Unmap()
		if !ipInPrefixes(addr, trusted) {
			return addr.String()
		}
	}
	return remoteAddr.String()
}

func remoteAddrIP(remoteAddr string) string {
	host, _, err := net.SplitHostPort(strings.TrimSpace(remoteAddr))
	if err == nil {
		remoteAddr = host
	}
	remoteAddr = strings.Trim(remoteAddr, "[]")
	if addr, err := netip.ParseAddr(remoteAddr); err == nil {
		return addr.Unmap().String()
	}
	return strings.TrimSpace(remoteAddr)
}

func ipInPrefixes(addr netip.Addr, prefixes []netip.Prefix) bool {
	for _, p := range prefixes {
		if p.Contains(addr) {
			return true
		}
	}
	return false
}

// DecodeJSON reads and decodes a single JSON object from the request body.
// DisallowUnknownFields catches typos/unexpected input early, which is useful for
// API clients and prevents silently ignoring bad fields.
func DecodeJSON(r *http.Request, dst any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return err
	}
	// Decode succeeds on the first JSON value; ensure there isn't a second object.
	if err := dec.Decode(&struct{}{}); err == nil {
		return errors.New("request body must contain a single JSON object")
	}
	return nil
}
