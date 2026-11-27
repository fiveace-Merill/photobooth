package admin

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/fiveace-Merill/photobooth/internal/notify"
	"github.com/fiveace-Merill/photobooth/internal/platform/httpx"
	"github.com/jackc/pgx/v5/pgtype"
)

func (h *Handler) handleInvitationsList(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	items, err := h.svc.ListInvitations(r.Context(), status)
	if err != nil {
		h.writeErr(w, err)
		return
	}
	out := make([]map[string]any, 0, len(items))
	for _, inv := range items {
		out = append(out, invitationJSON(inv))
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": out})
}

func (h *Handler) handleInvitationCreate(w http.ResponseWriter, r *http.Request) {
	admin, _ := AdminFromContext(r.Context())
	var req struct {
		Email, PlanID, Note string
		Days                int
		Send                bool
	}
	if err := decodeJSON(r, &req, 8<<10); err != nil {
		httpx.ErrorCode(w, http.StatusBadRequest, "invalid_json", "Invalid request body")
		return
	}
	if req.Days <= 0 {
		req.Days = 7
	}
	emailStatus := "not_sent"
	if req.Send {
		if h.sender == nil {
			emailStatus = "no_sender"
		} else if !h.inviteEmailLimiter.Allow("admin:" + uuidString(admin.ID)) {
			emailStatus = "rate_limited"
		}
	}
	auditMeta := map[string]any{"email": maskEmail(req.Email), "planId": strings.TrimSpace(req.PlanID), "send": req.Send, "emailStatus": emailStatus}
	raw, inv, err := h.svc.CreateInvitation(r.Context(), req.Email, req.PlanID, time.Duration(req.Days)*24*time.Hour, req.Note, admin.ID, AuditEntry{AdminID: admin.ID, Actor: "admin", Action: "invitation.created", TargetType: "invitation", Meta: auditMeta, IP: h.clientIP(r)})
	if err != nil {
		h.writeErr(w, err)
		return
	}
	url := h.publicAppURL + "/signup?invite=" + raw
	if req.Send && emailStatus == "not_sent" && h.sender != nil {
		if err := h.sendInvitationEmail(r.Context(), inv.Email, url, inv.ExpiresAt); err != nil {
			emailStatus = "failed"
		} else {
			emailStatus = "sent"
		}
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{"url": url, "expiresAt": inv.ExpiresAt.UTC(), "emailStatus": emailStatus, "invitation": invitationJSON(inv)})
}

func (h *Handler) handleInvitationRevoke(w http.ResponseWriter, r *http.Request) {
	admin, _ := AdminFromContext(r.Context())
	id, err := parseUUIDParam(r.PathValue("id"))
	if err != nil {
		httpx.ErrorCode(w, http.StatusNotFound, "not_found", "not found")
		return
	}
	inv, err := h.svc.RevokeInvitation(r.Context(), id, AuditEntry{AdminID: admin.ID, Actor: "admin", Action: "invitation.revoked", TargetType: "invitation", TargetID: uuidString(id), Meta: map[string]any{}, IP: h.clientIP(r)})
	if err != nil {
		h.writeErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, invitationJSON(inv))
}

func (h *Handler) handleInvitationResend(w http.ResponseWriter, r *http.Request) {
	admin, _ := AdminFromContext(r.Context())
	id, err := parseUUIDParam(r.PathValue("id"))
	if err != nil {
		httpx.ErrorCode(w, http.StatusNotFound, "not_found", "not found")
		return
	}
	var req struct {
		Days int
		Send bool
	}
	_ = decodeJSON(r, &req, 8<<10)
	if req.Days <= 0 {
		req.Days = 7
	}
	emailStatus := "not_sent"
	if req.Send {
		if h.sender == nil {
			emailStatus = "no_sender"
		} else if !h.inviteEmailLimiter.Allow("admin:" + uuidString(admin.ID)) {
			emailStatus = "rate_limited"
		}
	}
	raw, inv, err := h.svc.ResendInvitation(r.Context(), id, time.Duration(req.Days)*24*time.Hour, admin.ID, AuditEntry{AdminID: admin.ID, Actor: "admin", Action: "invitation.resent", TargetType: "invitation", Meta: map[string]any{"previousInvitationId": uuidString(id), "send": req.Send, "emailStatus": emailStatus}, IP: h.clientIP(r)})
	if err != nil {
		h.writeErr(w, err)
		return
	}
	url := h.publicAppURL + "/signup?invite=" + raw
	if req.Send && emailStatus == "not_sent" && h.sender != nil {
		if err := h.sendInvitationEmail(r.Context(), inv.Email, url, inv.ExpiresAt); err != nil {
			emailStatus = "failed"
		} else {
			emailStatus = "sent"
		}
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{"url": url, "expiresAt": inv.ExpiresAt.UTC(), "emailStatus": emailStatus, "invitation": invitationJSON(inv)})
}

func (h *Handler) handlePhotographersList(w http.ResponseWriter, r *http.Request) {
	limit, offset := pageParams(r)
	items, total, err := h.svc.ListPhotographers(r.Context(), r.URL.Query().Get("q"), r.URL.Query().Get("status"), limit, offset)
	if err != nil {
		h.writeErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items, "total": total, "limit": limit, "offset": offset})
}

func (h *Handler) handlePhotographerGet(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r.PathValue("id"))
	if err != nil {
		httpx.ErrorCode(w, http.StatusNotFound, "not_found", "not found")
		return
	}
	item, err := h.svc.GetPhotographer(r.Context(), id)
	if err != nil {
		h.writeErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, item)
}

func (h *Handler) handlePhotographerSuspend(w http.ResponseWriter, r *http.Request) {
	admin, _ := AdminFromContext(r.Context())
	id, err := parseUUIDParam(r.PathValue("id"))
	if err != nil {
		httpx.ErrorCode(w, http.StatusNotFound, "not_found", "not found")
		return
	}
	var req struct {
		Reason string `json:"reason"`
	}
	if err := decodeJSON(r, &req, 8<<10); err != nil {
		httpx.ErrorCode(w, http.StatusBadRequest, "invalid_json", "Invalid request body")
		return
	}
	if err := h.svc.SuspendPhotographer(r.Context(), id, req.Reason, admin.ID, h.clientIP(r)); err != nil {
		h.writeErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) handlePhotographerUnsuspend(w http.ResponseWriter, r *http.Request) {
	admin, _ := AdminFromContext(r.Context())
	id, err := parseUUIDParam(r.PathValue("id"))
	if err != nil {
		httpx.ErrorCode(w, http.StatusNotFound, "not_found", "not found")
		return
	}
	if err := h.svc.UnsuspendPhotographer(r.Context(), id, admin.ID, h.clientIP(r)); err != nil {
		h.writeErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) handlePhotographerPlan(w http.ResponseWriter, r *http.Request) {
	admin, _ := AdminFromContext(r.Context())
	id, err := parseUUIDParam(r.PathValue("id"))
	if err != nil {
		httpx.ErrorCode(w, http.StatusNotFound, "not_found", "not found")
		return
	}
	var req struct {
		PlanID string `json:"planId"`
	}
	if err := decodeJSON(r, &req, 8<<10); err != nil {
		httpx.ErrorCode(w, http.StatusBadRequest, "invalid_json", "Invalid request body")
		return
	}
	if err := h.svc.ChangePhotographerPlan(r.Context(), id, req.PlanID, admin.ID, h.clientIP(r)); err != nil {
		h.writeErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) handleWaitlistInvite(w http.ResponseWriter, r *http.Request) {
	admin, _ := AdminFromContext(r.Context())
	id, err := parseUUIDParam(r.PathValue("id"))
	if err != nil {
		httpx.ErrorCode(w, http.StatusNotFound, "not_found", "not found")
		return
	}
	var req struct {
		Email, PlanID string
		Days          int
		Send          bool
	}
	if err := decodeJSON(r, &req, 8<<10); err != nil {
		httpx.ErrorCode(w, http.StatusBadRequest, "invalid_json", "Invalid request body")
		return
	}
	if req.Days <= 0 {
		req.Days = 7
	}
	raw, inv, emailStatus, err := h.svc.InviteWaitlist(r.Context(), h.waitlist, id, req.Email, req.PlanID, time.Duration(req.Days)*24*time.Hour, req.Send, admin.ID, h.clientIP(r), h.sender, h.fromName, h.fromAddr, h.publicAppURL, h.inviteEmailLimiter)
	if err != nil {
		h.writeErr(w, err)
		return
	}
	url := h.publicAppURL + "/signup?invite=" + raw
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{"url": url, "expiresAt": inv.ExpiresAt.UTC(), "emailStatus": emailStatus})
}

func (h *Handler) sendInvitationEmail(ctx context.Context, to, url string, expiresAt time.Time) error {
	if h.sender == nil {
		return errors.New("no sender")
	}
	subject := "Your Photobooth invitation"
	text := "You've been invited to Photobooth.\n\nAccept your invitation: " + url + "\n\nThis link expires on " + expiresAt.UTC().Format("2 January 2006") + ".\n\nYou're receiving this because you requested early access to Photobooth."
	html := "<p>You've been invited to Photobooth.</p><p><a href=\"" + url + "\">Accept your invitation</a></p><p>This link expires on " + expiresAt.UTC().Format("2 January 2006") + ".</p><p>You're receiving this because you requested early access to Photobooth.</p>"
	_, err := h.sender.Send(ctx, notify.Message{FromName: h.fromName, FromAddr: h.fromAddr, To: to, Subject: subject, Text: text, HTML: html})
	return err
}

func invitationJSON(inv Invitation) map[string]any {
	return map[string]any{"id": uuidString(inv.ID), "email": inv.Email, "planId": inv.PlanID, "note": inv.Note, "expiresAt": inv.ExpiresAt.UTC(), "createdAt": inv.CreatedAt.UTC(), "usedAt": inv.UsedAt, "revokedAt": inv.RevokedAt, "status": inv.Status}
}

var _ = strings.TrimSpace
var _ = pgtype.UUID{}
