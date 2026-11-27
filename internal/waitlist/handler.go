package waitlist

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/netip"
	"time"

	"github.com/fiveace-Merill/photobooth/internal/platform/httpx"
	"github.com/fiveace-Merill/photobooth/internal/platform/ratelimit"
)

type Handler struct {
	svc            *Service
	limiter        *ratelimit.Limiter
	trustedProxies []netip.Prefix
}

func NewHandler(svc *Service, trustedProxies ...[]netip.Prefix) *Handler {
	var tp []netip.Prefix
	if len(trustedProxies) > 0 {
		tp = append([]netip.Prefix(nil), trustedProxies[0]...)
	}
	return &Handler{svc: svc, limiter: ratelimit.New(20, time.Minute), trustedProxies: tp}
}

func (h *Handler) SetLimiter(l *ratelimit.Limiter) { h.limiter = l }

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/waitlist", h.handleSubmit)
}

type submitRequest struct {
	Name            string `json:"name"`
	Contact         string `json:"contact"`
	PhotographyType string `json:"photographyType"`
	ShootsPerMonth  string `json:"shootsPerMonth"`
	Consent         bool   `json:"consent"`
	Website         string `json:"website"`
}

func (h *Handler) handleSubmit(w http.ResponseWriter, r *http.Request) {
	ip := httpx.ClientIP(r, h.trustedProxies)
	if h.limiter != nil && !h.limiter.Allow(ip) {
		httpx.ErrorCode(w, http.StatusTooManyRequests, "rate_limited", "Too many requests")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4<<10)
	defer r.Body.Close()
	var req submitRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		httpx.ErrorCode(w, http.StatusBadRequest, "invalid_json", "Invalid request body")
		return
	}
	_, err := h.svc.Submit(r.Context(), SubmitInput{
		Name: req.Name, Contact: req.Contact, PhotographyType: req.PhotographyType, ShootsPerMonth: req.ShootsPerMonth, Consent: req.Consent, Website: req.Website,
	})
	if err != nil {
		if errors.Is(err, ErrInvalidInput) {
			httpx.ErrorCode(w, http.StatusBadRequest, "invalid_input", "Check the form and try again.")
			return
		}
		httpx.ErrorCode(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	httpx.WriteJSON(w, http.StatusAccepted, map[string]any{"ok": true})
}
