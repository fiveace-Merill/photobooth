-- Queries for the auth package. sqlc turns these into Go in internal/auth/store.
-- Sessions are auth-owned; they are not tenant-scoped via RLS like client data.

-- name: CreatePhotographer :one
INSERT INTO photographers (email, password_hash, business_name, slug)
VALUES ($1, $2, $3, $4)
RETURNING id, email, password_hash, business_name, slug, phone, plan_id, storage_used_bytes, status, created_at, updated_at;

-- name: GetPhotographerByEmail :one
SELECT id, email, password_hash, business_name, slug, phone, plan_id, storage_used_bytes, status, created_at, updated_at
FROM photographers
WHERE email = $1;

-- name: CreateSession :one
INSERT INTO sessions (token_hash, photographer_id, expires_at)
VALUES ($1, $2, $3)
RETURNING token_hash, photographer_id, expires_at, created_at;

-- name: GetPhotographerBySessionTokenHash :one
SELECT p.id, p.email, p.password_hash, p.business_name, p.slug, p.phone, p.plan_id, p.storage_used_bytes, p.status, p.created_at, p.updated_at
FROM sessions s
JOIN photographers p ON p.id = s.photographer_id
WHERE s.token_hash = $1
  AND s.expires_at > now()
  AND p.status = 'active';

-- name: DeleteSession :exec
DELETE FROM sessions WHERE token_hash = $1;
