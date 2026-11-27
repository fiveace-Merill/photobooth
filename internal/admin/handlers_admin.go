package admin

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/fiveace-Merill/photobooth/internal/admin/store"
	"github.com/fiveace-Merill/photobooth/internal/platform/httpx"
	waitlistpkg "github.com/fiveace-Merill/photobooth/internal/waitlist"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func (h *Handler) handleOverview(w http.ResponseWriter, r *http.Request) {
	stats, waitlistNew, pendingInv, err := h.svc.Overview(r.Context())
	if err != nil {
		httpx.ErrorCode(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"photographers": map[string]any{"total": stats.PhotographersTotal, "active": stats.PhotographersActive, "suspended": stats.PhotographersSuspended},
		"shootsTotal":   stats.ShootsTotal, "photosTotal": stats.PhotosTotal, "shootsDeliveredLast30Days": stats.ShootsDeliveredLast30Days, "storageUsedBytesTotal": stats.StorageUsedBytesTotal,
		"waitlistNew": waitlistNew, "pendingInvitations": pendingInv,
	})
}

func (h *Handler) handleWaitlistList(w http.ResponseWriter, r *http.Request) {
	limit, offset := pageParams(r)
	res, err := h.waitlist.List(r.Context(), r.URL.Query().Get("status"), r.URL.Query().Get("q"), limit, offset)
	if err != nil {
		httpx.ErrorCode(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	items := make([]map[string]any, 0, len(res.Items))
	for _, e := range res.Items {
		items = append(items, waitlistEntryJSON(e))
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items, "total": res.Total, "limit": res.Limit, "offset": res.Offset})
}

func (h *Handler) handleWaitlistGet(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r.PathValue("id"))
	if err != nil {
		httpx.ErrorCode(w, http.StatusNotFound, "not_found", "not found")
		return
	}
	e, err := h.waitlist.GetByID(r.Context(), id)
	if err != nil {
		h.writeErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, waitlistEntryJSON(e))
}

func (h *Handler) handleWaitlistPatch(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r.PathValue("id"))
	if err != nil {
		httpx.ErrorCode(w, http.StatusNotFound, "not_found", "not found")
		return
	}
	var req struct {
		Notes *string `json:"notes"`
		Email *string `json:"email"`
		Phone *string `json:"phone"`
	}
	if err := decodeJSON(r, &req, 8<<10); err != nil {
		httpx.ErrorCode(w, http.StatusBadRequest, "invalid_json", "Invalid request body")
		return
	}
	admin, _ := AdminFromContext(r.Context())
	e, err := h.waitlist.Patch(r.Context(), id, waitlistpkg.PatchInput{Notes: req.Notes, Email: req.Email, Phone: req.Phone}, func(ctx context.Context, tx pgx.Tx) error {
		return (AuditEntry{AdminID: admin.ID, Actor: "admin", Action: "waitlist.updated", TargetType: "waitlist_entry", TargetID: uuidString(id), Meta: map[string]any{"status": "updated"}, IP: h.clientIP(r)}).Write(ctx, store.New(tx))
	})
	if err != nil {
		h.writeErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, waitlistEntryJSON(e))
}

func (h *Handler) handleWaitlistStatus(status string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseUUIDParam(r.PathValue("id"))
		if err != nil {
			httpx.ErrorCode(w, http.StatusNotFound, "not_found", "not found")
			return
		}
		admin, _ := AdminFromContext(r.Context())
		e, err := h.waitlist.SetStatus(r.Context(), id, status, func(ctx context.Context, tx pgx.Tx) error {
			return (AuditEntry{AdminID: admin.ID, Actor: "admin", Action: "waitlist." + status, TargetType: "waitlist_entry", TargetID: uuidString(id), Meta: map[string]any{"status": status}, IP: h.clientIP(r)}).Write(ctx, store.New(tx))
		})
		if err != nil {
			h.writeErr(w, err)
			return
		}
		httpx.WriteJSON(w, http.StatusOK, waitlistEntryJSON(e))
	}
}

func (h *Handler) handleWaitlistDelete(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r.PathValue("id"))
	if err != nil {
		httpx.ErrorCode(w, http.StatusNotFound, "not_found", "not found")
		return
	}
	admin, _ := AdminFromContext(r.Context())
	if err := h.waitlist.Delete(r.Context(), id, func(ctx context.Context, tx pgx.Tx) error {
		return (AuditEntry{AdminID: admin.ID, Actor: "admin", Action: "waitlist.deleted", TargetType: "waitlist_entry", TargetID: uuidString(id), Meta: map[string]any{}, IP: h.clientIP(r)}).Write(ctx, store.New(tx))
	}); err != nil {
		h.writeErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) handleWaitlistExport(w http.ResponseWriter, r *http.Request) {
	admin, _ := AdminFromContext(r.Context())
	rows, err := h.waitlist.ListExport(r.Context(), func(ctx context.Context, tx pgx.Tx) error {
		return (AuditEntry{AdminID: admin.ID, Actor: "admin", Action: "waitlist.exported", TargetType: "waitlist", TargetID: "", Meta: map[string]any{"format": "csv"}, IP: h.clientIP(r)}).Write(ctx, store.New(tx))
	})
	if err != nil {
		httpx.ErrorCode(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=\"waitlist.csv\"")
	cw := csv.NewWriter(w)
	_ = cw.Write([]string{"id", "name", "contact_email", "contact_phone", "photography_type", "shoots_per_month", "consent_at", "consent_version", "status", "invitation_id", "invited_at", "notes", "created_at", "updated_at"})
	for _, e := range rows {
		invited := ""
		if e.InvitedAt != nil {
			invited = e.InvitedAt.Format(time.RFC3339)
		}
		invID := ""
		if e.InvitationID.Valid {
			invID = uuidString(e.InvitationID)
		}
		_ = cw.Write([]string{neutralizeCSVCell(uuidString(e.ID)), neutralizeCSVCell(e.Name), neutralizeCSVCell(e.ContactEmail), neutralizeCSVCell(e.ContactPhone), neutralizeCSVCell(e.PhotographyType), neutralizeCSVCell(e.ShootsPerMonth), neutralizeCSVCell(e.ConsentAt.Format(time.RFC3339)), neutralizeCSVCell(e.ConsentVersion), neutralizeCSVCell(e.Status), neutralizeCSVCell(invID), neutralizeCSVCell(invited), neutralizeCSVCell(e.Notes), neutralizeCSVCell(e.CreatedAt.Format(time.RFC3339)), neutralizeCSVCell(e.UpdatedAt.Format(time.RFC3339))})
	}
	cw.Flush()
}

func (h *Handler) handleAuditList(w http.ResponseWriter, r *http.Request) {
	limit, offset := pageParams(r)
	rows, total, err := h.svc.AuditList(r.Context(), r.URL.Query().Get("action"), limit, offset)
	if err != nil {
		httpx.ErrorCode(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	items := make([]map[string]any, 0, len(rows))
	for _, a := range rows {
		items = append(items, map[string]any{"id": uuidString(a.ID), "adminId": uuidString(a.AdminID), "actor": a.Actor, "action": a.Action, "targetType": a.TargetType, "targetId": textOrEmpty(a.TargetID), "meta": json.RawMessage(a.Meta), "ip": textOrEmpty(a.Ip), "createdAt": a.CreatedAt.Time.UTC()})
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items, "total": total, "limit": limit, "offset": offset})
}

func waitlistEntryJSON(e waitlistpkg.Entry) map[string]any {
	var invitedAt any
	if e.InvitedAt != nil {
		invitedAt = e.InvitedAt.UTC()
	}
	var invID any
	if e.InvitationID.Valid {
		invID = uuidString(e.InvitationID)
	}
	return map[string]any{"id": uuidString(e.ID), "name": e.Name, "contactEmail": e.ContactEmail, "contactPhone": e.ContactPhone, "photographyType": e.PhotographyType, "shootsPerMonth": e.ShootsPerMonth, "consentAt": e.ConsentAt.UTC(), "consentVersion": e.ConsentVersion, "status": e.Status, "invitationId": invID, "invitedAt": invitedAt, "notes": e.Notes, "createdAt": e.CreatedAt.UTC(), "updatedAt": e.UpdatedAt.UTC()}
}

func textOrEmpty(t pgtype.Text) string {
	if t.Valid {
		return t.String
	}
	return ""
}

func pageParams(r *http.Request) (int32, int32) {
	limit := int32(25)
	offset := int32(0)
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = int32(n)
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			offset = int32(n)
		}
	}
	if limit <= 0 {
		limit = 25
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

var _ = strings.TrimSpace
var _ = store.Admin{}
