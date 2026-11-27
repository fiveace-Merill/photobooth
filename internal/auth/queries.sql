-- Queries for the auth package. sqlc turns these into Go in internal/auth/store.
-- Sessions/invitations are auth-owned; they are not tenant-scoped via RLS.

-- name: CreatePhotographer :one
INSERT INTO photographers (email, password_hash, business_name, slug)
VALUES ($1, $2, $3, $4)
RETURNING id, email, password_hash, business_name, slug, phone, plan_id, storage_used_bytes, status, created_at, updated_at;

-- name: CreatePhotographerWithPlan :one
INSERT INTO photographers (email, password_hash, business_name, slug, plan_id)
VALUES ($1, $2, $3, $4, $5)
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

-- name: UpdatePhotographerLastLoginAt :exec
UPDATE photographers
SET last_login_at = now()
WHERE id = $1
  AND (last_login_at IS NULL OR last_login_at < now() - interval '1 minute');

-- name: CreateInvitation :one
INSERT INTO invitations (email, token_hash, plan_id, note, expires_at, created_by)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, email, token_hash, plan_id, note, expires_at, created_at, created_by, used_at, photographer_id, revoked_at;

-- name: GetInvitationByID :one
SELECT id, email, token_hash, plan_id, note, expires_at, created_at, created_by, used_at, photographer_id, revoked_at
FROM invitations
WHERE id = $1;

-- name: GetInvitationByTokenHash :one
SELECT id, email, token_hash, plan_id, note, expires_at, created_at, created_by, used_at, photographer_id, revoked_at
FROM invitations
WHERE token_hash = $1;

-- name: LockInvitationByTokenHash :one
SELECT id, email, token_hash, plan_id, note, expires_at, created_at, created_by, used_at, photographer_id, revoked_at
FROM invitations
WHERE token_hash = $1
FOR UPDATE;

-- name: ConsumeInvitation :one
UPDATE invitations
SET used_at = now(), photographer_id = $2
WHERE id = $1
  AND used_at IS NULL
  AND revoked_at IS NULL
  AND expires_at > now()
RETURNING id, email, token_hash, plan_id, note, expires_at, created_at, created_by, used_at, photographer_id, revoked_at;

-- name: RevokeInvitation :one
UPDATE invitations
SET revoked_at = now()
WHERE id = $1
  AND used_at IS NULL
  AND revoked_at IS NULL
RETURNING id, email, token_hash, plan_id, note, expires_at, created_at, created_by, used_at, photographer_id, revoked_at;

-- name: RevokeExpiredActiveInvitationsByEmail :exec
UPDATE invitations
SET revoked_at = now()
WHERE email = $1
  AND used_at IS NULL
  AND revoked_at IS NULL
  AND expires_at <= now();

-- name: LockActiveInvitationsByEmail :many
SELECT id, email, token_hash, plan_id, note, expires_at, created_at, created_by, used_at, photographer_id, revoked_at
FROM invitations
WHERE email = $1
  AND used_at IS NULL
  AND revoked_at IS NULL
FOR UPDATE;

-- name: ListInvitations :many
SELECT id, email, token_hash, plan_id, note, expires_at, created_at, created_by, used_at, photographer_id, revoked_at
FROM invitations
WHERE (
    CASE
        WHEN sqlc.narg('status')::text IS NULL OR sqlc.narg('status')::text = '' THEN TRUE
        WHEN sqlc.narg('status')::text = 'pending' THEN used_at IS NULL AND revoked_at IS NULL AND expires_at > now()
        WHEN sqlc.narg('status')::text = 'used' THEN used_at IS NOT NULL
        WHEN sqlc.narg('status')::text = 'revoked' THEN revoked_at IS NOT NULL
        WHEN sqlc.narg('status')::text = 'expired' THEN used_at IS NULL AND revoked_at IS NULL AND expires_at <= now()
        ELSE FALSE
    END
)
ORDER BY created_at DESC, id DESC;
