package httpx

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
)

// ErrorResponse is the consistent JSON shape returned by API errors.
// Keeping error responses uniform makes frontend handling predictable.
type ErrorResponse struct {
	Error string `json:"error"`
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
