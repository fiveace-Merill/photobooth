-- name: CreateAdmin :one
INSERT INTO admins (email, password_hash, status)
VALUES ($1, $2, 'active')
RETURNING id, email, password_hash, totp_secret_enc, totp_confirmed_at, status, last_login_at, created_at;

-- name: GetAdminByEmail :one
SELECT id, email, password_hash, totp_secret_enc, totp_confirmed_at, status, last_login_at, created_at
FROM admins WHERE email = $1;

-- name: GetAdminByID :one
SELECT id, email, password_hash, totp_secret_enc, totp_confirmed_at, status, last_login_at, created_at
FROM admins WHERE id = $1;

-- name: SetAdminDisabled :execrows
UPDATE admins SET status = 'disabled' WHERE email = $1 AND status <> 'disabled';

-- name: SetAdminTOTPSecretEnc :exec
UPDATE admins SET totp_secret_enc = $2, totp_confirmed_at = NULL WHERE id = $1;

-- name: ConfirmAdminTOTP :exec
UPDATE admins SET totp_confirmed_at = now() WHERE id = $1;

-- name: ClearAdminTOTP :execrows
UPDATE admins SET totp_secret_enc = NULL, totp_confirmed_at = NULL WHERE email = $1;

-- name: UpdateAdminPasswordHash :exec
UPDATE admins SET password_hash = $2 WHERE id = $1;

-- name: UpdateAdminLastLoginAt :exec
UPDATE admins SET last_login_at = now() WHERE id = $1 AND (last_login_at IS NULL OR last_login_at < now() - interval '1 minute');

-- name: CreateAdminSession :one
INSERT INTO admin_sessions (token_hash, admin_id, pending, last_seen_at, expires_at)
VALUES ($1, $2, $3, now(), $4)
RETURNING token_hash, admin_id, pending, created_at, last_seen_at, expires_at;

-- name: GetAdminSessionByHash :one
SELECT s.token_hash, s.admin_id, s.pending, s.created_at, s.last_seen_at, s.expires_at,
       a.id, a.email, a.password_hash, a.totp_secret_enc, a.totp_confirmed_at, a.status, a.last_login_at, a.created_at AS admin_created_at
FROM admin_sessions s
JOIN admins a ON a.id = s.admin_id
WHERE s.token_hash = $1;

-- name: TouchAdminSession :exec
UPDATE admin_sessions SET last_seen_at = now() WHERE token_hash = $1 AND last_seen_at < now() - interval '1 minute';

-- name: PromoteAdminSession :exec
UPDATE admin_sessions SET pending = false, expires_at = $2, last_seen_at = now() WHERE token_hash = $1;

-- name: DeleteAdminSession :exec
DELETE FROM admin_sessions WHERE token_hash = $1;

-- name: DeleteAllAdminSessions :exec
DELETE FROM admin_sessions WHERE admin_id = $1;

-- name: DeleteOtherAdminSessions :exec
DELETE FROM admin_sessions WHERE admin_id = $1 AND token_hash <> $2;

-- name: DeleteRecoveryCodesByAdmin :exec
DELETE FROM admin_recovery_codes WHERE admin_id = $1;

-- name: InsertRecoveryCode :exec
INSERT INTO admin_recovery_codes (admin_id, code_hash) VALUES ($1, $2);

-- name: ConsumeRecoveryCode :execrows
UPDATE admin_recovery_codes SET used_at = now()
WHERE admin_id = $1 AND code_hash = $2 AND used_at IS NULL;

-- name: InsertAuditLog :exec
INSERT INTO admin_audit_log (admin_id, actor, action, target_type, target_id, meta, ip)
VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: ListAuditLog :many
SELECT id, admin_id, actor, action, target_type, target_id, meta, ip, created_at
FROM admin_audit_log
WHERE ($1::text = '' OR action = $1)
ORDER BY created_at DESC, id DESC
LIMIT $2 OFFSET $3;

-- name: CountAuditLog :one
SELECT COUNT(*)::bigint FROM admin_audit_log WHERE ($1::text = '' OR action = $1);

-- name: AdminPlatformStats :one
SELECT photographers_total, photographers_active, photographers_suspended, shoots_total, photos_total, shoots_delivered_last_30_days, storage_used_bytes_total
FROM admin_platform_stats() AS s(photographers_total, photographers_active, photographers_suspended, shoots_total, photos_total, shoots_delivered_last_30_days, storage_used_bytes_total);

-- name: ListPhotographers :many
SELECT p.id, p.email, p.business_name, p.slug, p.plan_id, p.status, p.storage_used_bytes, p.created_at, p.last_login_at, p.suspended_at, p.suspended_reason,
       pl.storage_quota_bytes
FROM photographers p
JOIN plans pl ON pl.id = p.plan_id
WHERE ($1::text = '' OR p.email ILIKE '%' || $1 || '%' OR p.business_name ILIKE '%' || $1 || '%' OR p.slug ILIKE '%' || $1 || '%')
  AND ($2::text = '' OR p.status = $2)
ORDER BY p.created_at DESC, p.id DESC
LIMIT $3 OFFSET $4;

-- name: CountPhotographers :one
SELECT COUNT(*)::bigint
FROM photographers p
WHERE ($1::text = '' OR p.email ILIKE '%' || $1 || '%' OR p.business_name ILIKE '%' || $1 || '%' OR p.slug ILIKE '%' || $1 || '%')
  AND ($2::text = '' OR p.status = $2);

-- name: GetPhotographerForAdmin :one
SELECT p.id, p.email, p.business_name, p.slug, p.plan_id, p.status, p.storage_used_bytes, p.created_at, p.last_login_at, p.suspended_at, p.suspended_reason,
       pl.storage_quota_bytes
FROM photographers p
JOIN plans pl ON pl.id = p.plan_id
WHERE p.id = $1;

-- name: AdminPhotographerUsage :many
SELECT photographer_id, shoots, photos, delivered
FROM admin_photographer_usage($1::uuid[]) AS u(photographer_id, shoots, photos, delivered);

-- name: SuspendPhotographer :execrows
UPDATE photographers
SET status = 'suspended', suspended_at = now(), suspended_reason = $2, updated_at = now()
WHERE id = $1 AND status <> 'suspended';

-- name: UnsuspendPhotographer :execrows
UPDATE photographers
SET status = 'active', suspended_at = NULL, suspended_reason = NULL, updated_at = now()
WHERE id = $1 AND status = 'suspended';

-- name: UpdatePhotographerPlan :execrows
UPDATE photographers SET plan_id = $2, updated_at = now() WHERE id = $1;

-- name: DeletePhotographerSessions :exec
DELETE FROM sessions WHERE photographer_id = $1;

-- name: GetPlanByID :one
SELECT id, name, storage_quota_bytes FROM plans WHERE id = $1;

-- name: CountPendingInvitations :one
SELECT COUNT(*)::bigint FROM invitations WHERE used_at IS NULL AND revoked_at IS NULL AND expires_at > now();

-- name: CreateInvitation :one
INSERT INTO invitations (email, token_hash, plan_id, note, expires_at, created_by)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, email, token_hash, plan_id, note, expires_at, created_at, created_by, used_at, photographer_id, revoked_at;

-- name: GetInvitationByID :one
SELECT id, email, token_hash, plan_id, note, expires_at, created_at, created_by, used_at, photographer_id, revoked_at
FROM invitations WHERE id = $1;

-- name: RevokeInvitation :one
UPDATE invitations SET revoked_at = now()
WHERE id = $1 AND used_at IS NULL AND revoked_at IS NULL
RETURNING id, email, token_hash, plan_id, note, expires_at, created_at, created_by, used_at, photographer_id, revoked_at;

-- name: RevokeExpiredActiveInvitationsByEmail :exec
UPDATE invitations SET revoked_at = now()
WHERE email = $1 AND used_at IS NULL AND revoked_at IS NULL AND expires_at <= now();

-- name: LockActiveInvitationsByEmail :many
SELECT id, email, token_hash, plan_id, note, expires_at, created_at, created_by, used_at, photographer_id, revoked_at
FROM invitations
WHERE email = $1 AND used_at IS NULL AND revoked_at IS NULL
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
