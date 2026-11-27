package admin

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/fiveace-Merill/photobooth/internal/admin/store"
	"github.com/jackc/pgx/v5/pgtype"
)

type AuditWriter interface {
	InsertAuditLog(ctx context.Context, arg store.InsertAuditLogParams) error
}

type AuditEntry struct {
	AdminID    pgtype.UUID
	Actor      string
	Action     string
	TargetType string
	TargetID   string
	Meta       map[string]any
	IP         string
}

func (e AuditEntry) Write(ctx context.Context, q AuditWriter) error {
	meta := e.Meta
	if meta == nil {
		meta = map[string]any{}
	}
	b, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	var admin pgtype.UUID
	if e.AdminID.Valid {
		admin = e.AdminID
	}
	targetID := strings.TrimSpace(e.TargetID)
	var tid pgtype.Text
	if targetID != "" {
		tid = pgtype.Text{String: targetID, Valid: true}
	}
	ip := strings.TrimSpace(e.IP)
	var ipt pgtype.Text
	if ip != "" {
		ipt = pgtype.Text{String: ip, Valid: true}
	}
	actor := strings.TrimSpace(e.Actor)
	if actor == "" {
		actor = "admin"
	}
	return q.InsertAuditLog(ctx, store.InsertAuditLogParams{
		AdminID: admin, Actor: actor, Action: e.Action, TargetType: e.TargetType, TargetID: tid, Meta: b, Ip: ipt,
	})
}

func writeAudit(ctx context.Context, q AuditWriter, adminID pgtype.UUID, actor, action, targetType, targetID string, meta map[string]any, ip string) error {
	return (AuditEntry{AdminID: adminID, Actor: actor, Action: action, TargetType: targetType, TargetID: targetID, Meta: meta, IP: ip}).Write(ctx, q)
}

func maskEmail(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return ""
	}
	parts := strings.SplitN(v, "@", 2)
	if len(parts) != 2 || parts[0] == "" {
		return "***"
	}
	local := parts[0]
	if len(local) == 1 {
		return "*@" + parts[1]
	}
	return local[:1] + "***@" + parts[1]
}

func maskPhone(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return ""
	}
	if len(v) <= 4 {
		return "****"
	}
	return v[:4] + "***" + v[len(v)-2:]
}
