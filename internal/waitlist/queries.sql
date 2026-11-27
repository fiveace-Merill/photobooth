-- name: CreateWaitlistEntry :one
INSERT INTO waitlist_entries (
  name, contact_email, contact_phone, photography_type, shoots_per_month, consent_at, consent_version, status
) VALUES ($1,$2,$3,$4,$5,$6,$7,'new')
RETURNING id, name, contact_email, contact_phone, photography_type, shoots_per_month, consent_at, consent_version, status, invitation_id, invited_at, notes, created_at, updated_at;

-- name: GetWaitlistByEmail :one
SELECT id, name, contact_email, contact_phone, photography_type, shoots_per_month, consent_at, consent_version, status, invitation_id, invited_at, notes, created_at, updated_at
FROM waitlist_entries WHERE contact_email = $1;

-- name: GetWaitlistByPhone :one
SELECT id, name, contact_email, contact_phone, photography_type, shoots_per_month, consent_at, consent_version, status, invitation_id, invited_at, notes, created_at, updated_at
FROM waitlist_entries WHERE contact_phone = $1;

-- name: GetWaitlistByID :one
SELECT id, name, contact_email, contact_phone, photography_type, shoots_per_month, consent_at, consent_version, status, invitation_id, invited_at, notes, created_at, updated_at
FROM waitlist_entries WHERE id = $1;

-- name: ListWaitlist :many
SELECT id, name, contact_email, contact_phone, photography_type, shoots_per_month, consent_at, consent_version, status, invitation_id, invited_at, notes, created_at, updated_at
FROM waitlist_entries
WHERE ($1::text = '' OR status = $1)
  AND ($2::text = '' OR name ILIKE '%' || $2 || '%' OR contact_email ILIKE '%' || $2 || '%' OR contact_phone ILIKE '%' || $2 || '%' OR photography_type ILIKE '%' || $2 || '%')
ORDER BY created_at DESC, id DESC
LIMIT $3 OFFSET $4;

-- name: CountWaitlist :one
SELECT COUNT(*)::bigint FROM waitlist_entries
WHERE ($1::text = '' OR status = $1)
  AND ($2::text = '' OR name ILIKE '%' || $2 || '%' OR contact_email ILIKE '%' || $2 || '%' OR contact_phone ILIKE '%' || $2 || '%' OR photography_type ILIKE '%' || $2 || '%');

-- name: CountWaitlistNew :one
SELECT COUNT(*)::bigint FROM waitlist_entries WHERE status = 'new';

-- name: UpdateWaitlistNotesContact :one
UPDATE waitlist_entries
SET notes = $2, contact_email = $3, contact_phone = $4, updated_at = now()
WHERE id = $1
RETURNING id, name, contact_email, contact_phone, photography_type, shoots_per_month, consent_at, consent_version, status, invitation_id, invited_at, notes, created_at, updated_at;

-- name: SetWaitlistStatus :one
UPDATE waitlist_entries
SET status = $2, updated_at = now(),
    invited_at = CASE WHEN $2 = 'invited' THEN now() ELSE invited_at END,
    invitation_id = CASE WHEN $2 = 'invited' THEN $3 ELSE invitation_id END
WHERE id = $1
RETURNING id, name, contact_email, contact_phone, photography_type, shoots_per_month, consent_at, consent_version, status, invitation_id, invited_at, notes, created_at, updated_at;

-- name: MarkWaitlistInvited :one
UPDATE waitlist_entries
SET status = 'invited', invited_at = now(), invitation_id = $2, contact_email = COALESCE($3, contact_email), updated_at = now()
WHERE id = $1
RETURNING id, name, contact_email, contact_phone, photography_type, shoots_per_month, consent_at, consent_version, status, invitation_id, invited_at, notes, created_at, updated_at;

-- name: DeleteWaitlist :execrows
DELETE FROM waitlist_entries WHERE id = $1;

-- name: ListWaitlistExport :many
SELECT id, name, contact_email, contact_phone, photography_type, shoots_per_month, consent_at, consent_version, status, invitation_id, invited_at, notes, created_at, updated_at
FROM waitlist_entries
ORDER BY created_at DESC, id DESC;
