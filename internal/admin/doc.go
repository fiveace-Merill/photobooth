// Package admin implements the platform-operator control plane.
//
// Admins are separate from photographers: different table, cookie, session model,
// middleware, and routes. The admin surface exposes platform metadata and
// aggregates only (never clients, shoots, photos, delivery links, raw tokens, or
// notification contents), and it does not support impersonation.
//
// Admin tables are platform-owned and not tenant-scoped via RLS.
package admin
