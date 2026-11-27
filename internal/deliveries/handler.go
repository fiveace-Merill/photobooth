package deliveries

import (
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/fiveace-Merill/photobooth/internal/auth"
	"github.com/fiveace-Merill/photobooth/internal/platform/httpx"
)

type Handler struct {
	svc     *Service
	limiter *ipLimiter
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc, limiter: newIPLimiter(30, time.Minute)}
}

func (h *Handler) Register(mux *http.ServeMux, requireAuth func(http.HandlerFunc) http.HandlerFunc) {
	// Photographer endpoints
	mux.HandleFunc("POST /api/v1/shoots/{id}/deliver", requireAuth(h.deliver))
	mux.HandleFunc("POST /api/v1/shoots/{id}/link/regenerate", requireAuth(h.regenerate))
	mux.HandleFunc("POST /api/v1/shoots/{id}/link/revoke", requireAuth(h.revoke))
	mux.HandleFunc("POST /api/v1/shoots/{id}/mark-paid", requireAuth(h.markPaid))
	mux.HandleFunc("GET /api/v1/shoots/{id}/delivery", requireAuth(h.getDeliveryState))
	mux.HandleFunc("GET /api/v1/shoots/{id}/notifications", requireAuth(h.listNotifications))
	mux.HandleFunc("GET /api/v1/shoots/{id}/delivery/message-preview", requireAuth(h.getMessagePreview))
	// Public endpoints
	mux.HandleFunc("GET /api/v1/deliveries/{token}", h.rateLimit(h.getPublicDelivery))
	mux.HandleFunc("GET /api/v1/deliveries/{token}/photos/{photoId}/original", h.rateLimit(h.getOriginal))
	mux.HandleFunc("GET /api/v1/deliveries/{token}/download", h.rateLimit(h.downloadAll))
}

type sendRequest struct {
	Send *struct {
		Email bool `json:"email"`
	} `json:"send"`
}

func parseSendOptions(r *http.Request) (SendOptions, error) {
	if r.Body == nil || r.ContentLength == 0 {
		return SendOptions{}, nil
	}
	var req sendRequest
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&req); err != nil {
		if errors.Is(err, io.EOF) {
			return SendOptions{}, nil
		}
		return SendOptions{}, err
	}
	if req.Send == nil {
		return SendOptions{}, nil
	}
	return SendOptions{Email: req.Send.Email}, nil
}

func (h *Handler) deliver(w http.ResponseWriter, r *http.Request) {
	pid, shootID, ok := authIDs(w, r)
	if !ok {
		return
	}
	send, err := parseSendOptions(r)
	if err != nil {
		httpx.ErrorCode(w, http.StatusBadRequest, "invalid_json", "Invalid JSON body")
		return
	}
	res, err := h.svc.Deliver(r.Context(), pid, shootID, send)
	if err != nil {
		writeErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, res)
}

func (h *Handler) regenerate(w http.ResponseWriter, r *http.Request) {
	pid, shootID, ok := authIDs(w, r)
	if !ok {
		return
	}
	send, err := parseSendOptions(r)
	if err != nil {
		httpx.ErrorCode(w, http.StatusBadRequest, "invalid_json", "Invalid JSON body")
		return
	}
	res, err := h.svc.RegenerateLink(r.Context(), pid, shootID, send)
	if err != nil {
		writeErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, res)
}

func (h *Handler) revoke(w http.ResponseWriter, r *http.Request) {
	pid, shootID, ok := authIDs(w, r)
	if !ok {
		return
	}
	if err := h.svc.RevokeLink(r.Context(), pid, shootID); err != nil {
		writeErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) markPaid(w http.ResponseWriter, r *http.Request) {
	pid, shootID, ok := authIDs(w, r)
	if !ok {
		return
	}
	var in MarkPaidInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		httpx.ErrorCode(w, http.StatusBadRequest, "invalid_json", "Invalid JSON body")
		return
	}
	if err := h.svc.MarkPaid(r.Context(), pid, shootID, in); err != nil {
		writeErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) getDeliveryState(w http.ResponseWriter, r *http.Request) {
	pid, shootID, ok := authIDs(w, r)
	if !ok {
		return
	}
	res, err := h.svc.GetDeliveryState(r.Context(), pid, shootID)
	if err != nil {
		writeErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, res)
}

func (h *Handler) listNotifications(w http.ResponseWriter, r *http.Request) {
	pid, shootID, ok := authIDs(w, r)
	if !ok {
		return
	}
	res, err := h.svc.ListNotifications(r.Context(), pid, shootID)
	if err != nil {
		writeErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": res})
}

func (h *Handler) getMessagePreview(w http.ResponseWriter, r *http.Request) {
	pid, shootID, ok := authIDs(w, r)
	if !ok {
		return
	}
	res, err := h.svc.GetMessagePreview(r.Context(), pid, shootID)
	if err != nil {
		writeErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, res)
}

func (h *Handler) getPublicDelivery(w http.ResponseWriter, r *http.Request) {
	setPublicHeaders(w)
	token := r.PathValue("token")
	res, err := h.svc.GetPublicDelivery(r.Context(), token)
	if err != nil {
		writePublicErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, res)
}

func (h *Handler) getOriginal(w http.ResponseWriter, r *http.Request) {
	setPublicHeaders(w)
	token := r.PathValue("token")
	photoID, err := uuid.Parse(r.PathValue("photoId"))
	if err != nil {
		writePublicErr(w, ErrNotFound)
		return
	}
	res, err := h.svc.GetOriginalRedirect(r.Context(), token, photoID)
	if err != nil {
		writePublicErr(w, err)
		return
	}
	w.Header().Set("Location", res.URL)
	w.WriteHeader(http.StatusFound)
}

func (h *Handler) downloadAll(w http.ResponseWriter, r *http.Request) {
	setPublicHeaders(w)
	token := r.PathValue("token")
	info, photographerID, shootID, err := h.svc.GetZipInfo(r.Context(), token)
	if err != nil {
		writePublicErr(w, err)
		return
	}
	base := SanitizeZipBase(info.ShootTitle)
	cd := contentDisposition(base + ".zip")
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", cd)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	// Allow long streaming downloads; reverse proxy timeouts must also be generous.
	rc := http.NewResponseController(w)
	_ = rc.SetWriteDeadline(time.Now().Add(30 * time.Minute))
	if err := sWriteZip(h.svc, r, w, info); err != nil {
		h.svc.logger.Warn("delivery zip failed", "err", err)
		return
	}
	h.svc.LogDownloadEvent(r.Context(), photographerID, shootID)
}

func sWriteZip(svc *Service, r *http.Request, w http.ResponseWriter, info ZipInfo) error {
	return svc.WriteZip(r.Context(), w, info)
}

func authIDs(w http.ResponseWriter, r *http.Request) (uuid.UUID, uuid.UUID, bool) {
	p, ok := auth.PhotographerFromContext(r.Context())
	if !ok {
		httpx.ErrorCode(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return uuid.UUID{}, uuid.UUID{}, false
	}
	shootID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httpx.ErrorCode(w, http.StatusNotFound, "not_found", "Shoot not found")
		return uuid.UUID{}, uuid.UUID{}, false
	}
	return uuid.UUID(p.ID.Bytes), shootID, true
}

func setPublicHeaders(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
}

func writeErr(w http.ResponseWriter, err error) {
	if errors.Is(err, ErrNotFound) {
		httpx.ErrorCode(w, http.StatusNotFound, "not_found", "Not found")
		return
	}
	if status, code, msg, ok := AsServiceError(err); ok {
		httpx.ErrorCode(w, status, code, msg)
		return
	}
	httpx.ErrorCode(w, http.StatusInternalServerError, "internal_error", "Internal server error")
}

func writePublicErr(w http.ResponseWriter, err error) {
	// Invalid/revoked/expired/missing all return identical 404.
	if errors.Is(err, ErrNotFound) {
		httpx.ErrorCode(w, http.StatusNotFound, "not_found", "Not found")
		return
	}
	if status, code, msg, ok := AsServiceError(err); ok {
		if status == http.StatusNotFound {
			httpx.ErrorCode(w, http.StatusNotFound, "not_found", "Not found")
			return
		}
		httpx.ErrorCode(w, status, code, msg)
		return
	}
	httpx.ErrorCode(w, http.StatusInternalServerError, "internal_error", "Internal server error")
}

func (h *Handler) rateLimit(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r)
		if !h.limiter.allow(ip) {
			setPublicHeaders(w)
			httpx.ErrorCode(w, http.StatusTooManyRequests, "rate_limited", "Too many requests")
			return
		}
		next(w, r)
	}
}

type ipLimiter struct {
	mu     sync.Mutex
	hits   map[string][]time.Time
	limit  int
	window time.Duration
}

func newIPLimiter(limit int, window time.Duration) *ipLimiter {
	return &ipLimiter{hits: map[string][]time.Time{}, limit: limit, window: window}
}

func (l *ipLimiter) allow(ip string) bool {
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

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}
