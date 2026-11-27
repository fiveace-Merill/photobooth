package admin

import (
	"context"

	"github.com/fiveace-Merill/photobooth/internal/admin/store"
)

type contextKey string

const adminContextKey contextKey = "admin"
const adminSessionTokenHashKey contextKey = "admin_session_token_hash"

func WithAdmin(ctx context.Context, a store.Admin) context.Context {
	return context.WithValue(ctx, adminContextKey, a)
}

func AdminFromContext(ctx context.Context) (store.Admin, bool) {
	a, ok := ctx.Value(adminContextKey).(store.Admin)
	return a, ok
}

func WithSessionTokenHash(ctx context.Context, hash []byte) context.Context {
	cp := append([]byte(nil), hash...)
	return context.WithValue(ctx, adminSessionTokenHashKey, cp)
}

func SessionTokenHashFromContext(ctx context.Context) ([]byte, bool) {
	v, ok := ctx.Value(adminSessionTokenHashKey).([]byte)
	if !ok || len(v) == 0 {
		return nil, false
	}
	return append([]byte(nil), v...), true
}
