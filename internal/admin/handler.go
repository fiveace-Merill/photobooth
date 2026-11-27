package admin

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/netip"
	"strings"
	"time"

	"github.com/fiveace-Merill/photobooth/internal/notify"
	"github.com/fiveace-Merill/photobooth/internal/platform/httpx"
	"github.com/fiveace-Merill/photobooth/internal/platform/ratelimit"
	waitlistpkg "github.com/fiveace-Merill/photobooth/internal/waitlist"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type WaitlistService interface {
	List(ctx context.Context, status, q string, limit, offset int32) (waitlistpkg.ListResult, error)
	GetByID(ctx context.Context, id pgtype.UUID) (waitlistpkg.Entry, error)
	Patch(ctx context.Context, id pgtype.UUID, in waitlistpkg.PatchInput, audit waitlistpkg.AuditFn) (waitlistpkg.Entry, error)
	SetStatus(ctx context.Context, id pgtype.UUID, status string, audit waitlistpkg.AuditFn) (waitlistpkg.Entry, error)
	Delete(ctx context.Context, id pgtype.UUID, audit waitlistpkg.AuditFn) error
	ListExport(ctx context.Context, audit waitlistpkg.AuditFn) ([]waitlistpkg.Entry, error)
	CountNew(ctx context.Context) (int64, error)
	MarkInvitedInTx(ctx context.Context, tx pgx.Tx, id pgtype.UUID, invitationID pgtype.UUID, email string) (waitlistpkg.Entry, error)
}

type InvitationService interface {
	CreateAdminInvitation(ctx context.Context, email, planID string, ttl time.Duration, note string, createdBy pgtype.UUID) (string, InvitationView, error) // raw url token? We'll return raw token and build URL in handler.
}

type Handler struct {
	svc                *Service
	waitlist           WaitlistService
	sender             notify.EmailSender
	fromName           string
	fromAddr           string
	publicAppURL       string
	cookieSecure       bool
	loginLimiter       *ratelimit.Limiter
	loginEmailLimiter  *ratelimit.Limiter
	inviteEmailLimiter *ratelimit.Limiter
	trustedProxies     []netip.Prefix
}

func NewHandler(svc *Service, wl WaitlistService, sender notify.EmailSender, fromName, fromAddr, publicAppURL string, cookieSecure bool, trustedProxies []netip.Prefix) *Handler {
	return &Handler{svc: svc, waitlist: wl, sender: sender, fromName: fromName, fromAddr: fromAddr, publicAppURL: strings.TrimRight(publicAppURL, "/"), cookieSecure: cookieSecure,
		loginLimiter: ratelimit.New(5, 15*time.Minute), loginEmailLimiter: ratelimit.New(5, 15*time.Minute), inviteEmailLimiter: ratelimit.New(30, time.Hour), trustedProxies: append([]netip.Prefix(nil), trustedProxies...)}
}

type InvitationView struct {
	ID        string
	Email     string
	ExpiresAt time.Time
}

func (h *Handler) Register(mux *http.ServeMux, mw *Middleware) {
	wrap := func(hn http.Handler) http.Handler { return mw.Load(hn) }
	authJSON := func(fn http.HandlerFunc) http.Handler { return wrap(RequireSameOriginJSON(fn)) }
	adminJSON := func(fn http.HandlerFunc) http.Handler { return wrap(RequireAdmin(RequireSameOriginJSON(fn))) }
	adminGet := func(fn http.HandlerFunc) http.Handler { return wrap(RequireAdmin(fn)) }
	pendingJSON := func(fn http.HandlerFunc) http.Handler { return wrap(RequirePendingAdmin(RequireSameOriginJSON(fn))) }

	mux.Handle("POST /api/v1/admin/auth/login", wrap(RequireSameOriginJSON(http.HandlerFunc(h.handleLogin))))
	mux.Handle("POST /api/v1/admin/auth/totp/verify", pendingJSON(h.handleTOTPVerify))
	mux.Handle("POST /api/v1/admin/auth/totp/setup/start", pendingJSON(h.handleTOTPSetupStart))
	mux.Handle("POST /api/v1/admin/auth/totp/setup/confirm", pendingJSON(h.handleTOTPSetupConfirm))
	mux.Handle("POST /api/v1/admin/auth/logout", authJSON(h.handleLogout))
	mux.Handle("GET /api/v1/admin/auth/me", adminGet(h.handleMe))
	mux.Handle("POST /api/v1/admin/auth/password", adminJSON(h.handlePassword))

	mux.Handle("GET /api/v1/admin/overview", adminGet(h.handleOverview))
	mux.Handle("GET /api/v1/admin/waitlist", adminGet(h.handleWaitlistList))
	mux.Handle("GET /api/v1/admin/waitlist/export.csv", adminGet(h.handleWaitlistExport))
	mux.Handle("GET /api/v1/admin/waitlist/{id}", adminGet(h.handleWaitlistGet))
	mux.Handle("PATCH /api/v1/admin/waitlist/{id}", adminJSON(h.handleWaitlistPatch))
	mux.Handle("POST /api/v1/admin/waitlist/{id}/invite", adminJSON(h.handleWaitlistInvite))
	mux.Handle("POST /api/v1/admin/waitlist/{id}/decline", adminJSON(h.handleWaitlistStatus("declined")))
	mux.Handle("POST /api/v1/admin/waitlist/{id}/spam", adminJSON(h.handleWaitlistStatus("spam")))
	mux.Handle("DELETE /api/v1/admin/waitlist/{id}", adminJSON(h.handleWaitlistDelete))

	mux.Handle("GET /api/v1/admin/invitations", adminGet(h.handleInvitationsList))
	mux.Handle("POST /api/v1/admin/invitations", adminJSON(h.handleInvitationCreate))
	mux.Handle("POST /api/v1/admin/invitations/{id}/revoke", adminJSON(h.handleInvitationRevoke))
	mux.Handle("POST /api/v1/admin/invitations/{id}/resend", adminJSON(h.handleInvitationResend))

	mux.Handle("GET /api/v1/admin/photographers", adminGet(h.handlePhotographersList))
	mux.Handle("GET /api/v1/admin/photographers/{id}", adminGet(h.handlePhotographerGet))
	mux.Handle("POST /api/v1/admin/photographers/{id}/suspend", adminJSON(h.handlePhotographerSuspend))
	mux.Handle("POST /api/v1/admin/photographers/{id}/unsuspend", adminJSON(h.handlePhotographerUnsuspend))
	mux.Handle("PATCH /api/v1/admin/photographers/{id}/plan", adminJSON(h.handlePhotographerPlan))

	mux.Handle("GET /api/v1/admin/audit", adminGet(h.handleAuditList))
}

func (h *Handler) clientIP(r *http.Request) string {
	return httpx.ClientIP(r, h.trustedProxies)
}

func (h *Handler) handleLogin(w http.ResponseWriter, r *http.Request) {
	ip := h.clientIP(r)
	var req struct{ Email, Password string }
	if err := decodeJSON(r, &req, 8<<10); err != nil {
		httpx.ErrorCode(w, http.StatusBadRequest, "invalid_json", "Invalid request body")
		return
	}
	emailKey := strings.ToLower(strings.TrimSpace(req.Email))
	if !h.loginLimiter.Allow(ip) || !h.loginEmailLimiter.Allow("email:"+emailKey) {
		httpx.ErrorCode(w, http.StatusTooManyRequests, "rate_limited", "Too many requests")
		return
	}
	res, err := h.svc.Login(r.Context(), req.Email, req.Password, ip)
	if err != nil {
		if errors.Is(err, ErrInvalidCredentials) {
			httpx.ErrorCode(w, http.StatusUnauthorized, "invalid_credentials", "Invalid credentials")
			return
		}
		httpx.ErrorCode(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	maxAge := int(h.svc.sessionMax.Seconds())
	if res.Next != "ok" {
		maxAge = 300
	}
	SetAdminCookie(w, res.RawToken, h.cookieSecure, maxAge)
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"next": res.Next})
}

func (h *Handler) handleLogout(w http.ResponseWriter, r *http.Request) {
	c, _ := r.Cookie(SessionCookieName)
	if c != nil {
		_ = h.svc.Logout(r.Context(), c.Value)
	}
	clearAdminCookie(w, h.cookieSecure)
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) handleMe(w http.ResponseWriter, r *http.Request) {
	a, _ := AdminFromContext(r.Context())
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"id": uuidString(a.ID), "email": a.Email, "status": a.Status, "totpConfirmed": a.TotpConfirmedAt.Valid})
}

func (h *Handler) handleTOTPSetupStart(w http.ResponseWriter, r *http.Request) {
	a, _ := AdminFromContext(r.Context())
	secret, uri, err := h.svc.StartTOTPSetup(r.Context(), a)
	if err != nil {
		h.writeErr(w, err)
		return
	}
	_ = secret
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"otpauthUri": uri, "secret": secret})
}

func (h *Handler) handleTOTPSetupConfirm(w http.ResponseWriter, r *http.Request) {
	a, _ := AdminFromContext(r.Context())
	var req struct {
		Code string `json:"code"`
	}
	if err := decodeJSON(r, &req, 4<<10); err != nil {
		httpx.ErrorCode(w, http.StatusBadRequest, "invalid_json", "Invalid request body")
		return
	}
	codes, err := h.svc.ConfirmTOTPSetup(r.Context(), a, req.Code, AuditEntry{AdminID: a.ID, Actor: "admin", Action: "admin.totp.confirmed", TargetType: "admin", TargetID: uuidString(a.ID), Meta: map[string]any{}, IP: h.clientIP(r)})
	if err != nil {
		h.writeErr(w, err)
		return
	}
	hash, _ := SessionTokenHashFromContext(r.Context())
	if err := h.svc.PromoteSession(r.Context(), hash); err != nil {
		h.writeErr(w, err)
		return
	}
	if c, err := r.Cookie(SessionCookieName); err == nil {
		SetAdminCookie(w, c.Value, h.cookieSecure, int(h.svc.sessionMax.Seconds()))
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"recoveryCodes": codes})
}

func (h *Handler) handleTOTPVerify(w http.ResponseWriter, r *http.Request) {
	a, _ := AdminFromContext(r.Context())
	var req struct {
		Code         string `json:"code"`
		RecoveryCode string `json:"recoveryCode"`
	}
	if err := decodeJSON(r, &req, 4<<10); err != nil {
		httpx.ErrorCode(w, http.StatusBadRequest, "invalid_json", "Invalid request body")
		return
	}
	if err := h.svc.VerifyTOTPOrRecovery(r.Context(), a, req.Code, req.RecoveryCode); err != nil {
		h.writeErr(w, err)
		return
	}
	hash, _ := SessionTokenHashFromContext(r.Context())
	if err := h.svc.PromoteSession(r.Context(), hash); err != nil {
		h.writeErr(w, err)
		return
	}
	if c, err := r.Cookie(SessionCookieName); err == nil {
		SetAdminCookie(w, c.Value, h.cookieSecure, int(h.svc.sessionMax.Seconds()))
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) handlePassword(w http.ResponseWriter, r *http.Request) {
	a, _ := AdminFromContext(r.Context())
	hash, _ := SessionTokenHashFromContext(r.Context())
	var req struct{ Current, New string }
	if err := decodeJSON(r, &req, 8<<10); err != nil {
		httpx.ErrorCode(w, http.StatusBadRequest, "invalid_json", "Invalid request body")
		return
	}
	if err := h.svc.ChangePassword(r.Context(), a, req.Current, req.New, hash, h.clientIP(r)); err != nil {
		h.writeErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) writeErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrInvalidCredentials):
		httpx.ErrorCode(w, http.StatusUnauthorized, "invalid_credentials", "Invalid credentials")
	case errors.Is(err, ErrUnauthorized):
		httpx.ErrorCode(w, http.StatusUnauthorized, "unauthorized", "authentication required")
	case errors.Is(err, ErrNotFound), errors.Is(err, waitlistpkg.ErrNotFound):
		httpx.ErrorCode(w, http.StatusNotFound, "not_found", "not found")
	case errors.Is(err, ErrInvalidInput), errors.Is(err, waitlistpkg.ErrInvalidInput):
		httpx.ErrorCode(w, http.StatusBadRequest, "invalid_input", err.Error())
	case errors.Is(err, ErrRateLimited):
		httpx.ErrorCode(w, http.StatusTooManyRequests, "rate_limited", "Too many requests")
	default:
		httpx.ErrorCode(w, http.StatusInternalServerError, "internal", "internal server error")
	}
}

func decodeJSON(r *http.Request, dst any, max int64) error {
	r.Body = http.MaxBytesReader(nil, r.Body, max)
	defer r.Body.Close()
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}

func parseUUIDParam(s string) (pgtype.UUID, error) {
	var id pgtype.UUID
	err := id.Scan(s)
	return id, err
}
