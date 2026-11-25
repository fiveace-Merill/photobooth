package auth

import (
	"context"

	"github.com/fiveace-Merill/photobooth/internal/auth/store"
)

// contextKey is an unexported type used for context values in this package.
// Using a distinct type (not string) avoids collisions with other packages that
// might store values under the same key name.
type contextKey string

const photographerContextKey contextKey = "photographer"

// WithPhotographer stores the authenticated photographer in the request context.
func WithPhotographer(ctx context.Context, p store.Photographer) context.Context {
	return context.WithValue(ctx, photographerContextKey, p)
}

// PhotographerFromContext returns the authenticated photographer, if present.
// The bool return follows the common Go "comma ok" pattern.
func PhotographerFromContext(ctx context.Context) (store.Photographer, bool) {
	p, ok := ctx.Value(photographerContextKey).(store.Photographer)
	return p, ok
}
