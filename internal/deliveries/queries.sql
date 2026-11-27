-- name: GetShootDeliveryInfo :one
SELECT s.id, s.photographer_id, s.client_id, s.title, s.shoot_date, s.status, s.due_date,
       s.price_minor, s.currency, s.originals_unlocked_at, s.delivered_at,
       c.name AS client_name, COALESCE(c.email::text, '') AS client_email, c.phone AS client_phone,
       p.business_name AS studio_name, p.phone AS studio_phone, COALESCE(p.email::text, '') AS studio_email,
       pl.retention_days
FROM shoots s
JOIN clients c ON c.photographer_id = s.photographer_id AND c.id = s.client_id
JOIN photographers p ON p.id = s.photographer_id
JOIN plans pl ON pl.id = p.plan_id
WHERE s.photographer_id = $1 AND s.id = $2;

-- name: CountShootPhotosByStatus :one
SELECT
  COUNT(*) FILTER (WHERE status = 'ready')::int AS ready_count,
  COUNT(*) FILTER (WHERE status = 'processing')::int AS processing_count
FROM photos
WHERE photographer_id = $1 AND shoot_id = $2 AND deleted_at IS NULL;

-- name: GetActiveGalleryLink :one
SELECT id, photographer_id, shoot_id, token_hash, expires_at, revoked_at, last_viewed_at, created_at
FROM gallery_links
WHERE photographer_id = $1 AND shoot_id = $2 AND revoked_at IS NULL
ORDER BY created_at DESC
LIMIT 1;

-- name: GetLatestGalleryLink :one
SELECT id, photographer_id, shoot_id, token_hash, expires_at, revoked_at, last_viewed_at, created_at
FROM gallery_links
WHERE photographer_id = $1 AND shoot_id = $2
ORDER BY created_at DESC
LIMIT 1;

-- name: InsertGalleryLink :one
INSERT INTO gallery_links (photographer_id, shoot_id, token_hash, expires_at)
VALUES ($1, $2, $3, $4)
RETURNING id, photographer_id, shoot_id, token_hash, expires_at, revoked_at, last_viewed_at, created_at;

-- name: RevokeActiveGalleryLinks :execrows
UPDATE gallery_links
SET revoked_at = now()
WHERE photographer_id = $1 AND shoot_id = $2 AND revoked_at IS NULL;

-- name: MarkShootDelivered :one
UPDATE shoots
SET status = 'delivered',
    delivered_at = COALESCE(delivered_at, now()),
    originals_unlocked_at = CASE WHEN $3::bool THEN COALESCE(originals_unlocked_at, now()) ELSE originals_unlocked_at END,
    updated_at = now()
WHERE photographer_id = $1 AND id = $2
RETURNING id, photographer_id, client_id, title, shoot_date, status, due_date, price_minor, currency, originals_unlocked_at, delivered_at, created_at, updated_at;

-- name: MarkShootPaid :one
UPDATE shoots
SET originals_unlocked_at = COALESCE(originals_unlocked_at, now()), updated_at = now()
WHERE photographer_id = $1 AND id = $2
RETURNING id, photographer_id, client_id, title, shoot_date, status, due_date, price_minor, currency, originals_unlocked_at, delivered_at, created_at, updated_at;

-- name: CreateShootEvent :exec
INSERT INTO shoot_events (photographer_id, shoot_id, photo_id, kind, payload)
VALUES ($1, $2, $3, $4, $5);

-- name: ListShootEvents :many
SELECT id, shoot_id, photo_id, kind, payload, created_at
FROM shoot_events
WHERE photographer_id = $1 AND shoot_id = $2
ORDER BY created_at DESC
LIMIT $3;

-- name: GetDeliveryShootForPublic :one
SELECT s.id, s.photographer_id, s.client_id, s.title, s.shoot_date, s.status, s.due_date,
       s.price_minor, s.currency, s.originals_unlocked_at, s.delivered_at,
       p.business_name AS studio_name, p.phone AS studio_phone
FROM shoots s
JOIN photographers p ON p.id = s.photographer_id
WHERE s.photographer_id = $1 AND s.id = $2;

-- name: ListReadyPhotosForDelivery :many
SELECT id, shoot_id, filename, size_bytes, width, height, original_key, preview_key, thumb_key, sort_key, created_at
FROM photos
WHERE photographer_id = $1 AND shoot_id = $2 AND deleted_at IS NULL AND status = 'ready'
ORDER BY sort_key ASC, created_at ASC, id ASC;

-- name: GetReadyPhotoForDelivery :one
SELECT id, shoot_id, filename, size_bytes, width, height, original_key, preview_key, thumb_key
FROM photos
WHERE photographer_id = $1 AND shoot_id = $2 AND id = $3 AND deleted_at IS NULL AND status = 'ready';

-- name: TouchGalleryLinkViewed :execrows
UPDATE gallery_links
SET last_viewed_at = now()
WHERE photographer_id = $1 AND shoot_id = $2 AND token_hash = $3 AND revoked_at IS NULL
  AND (last_viewed_at IS NULL OR last_viewed_at < now() - interval '1 minute');

-- name: GetPhotographerPhone :one
SELECT phone FROM photographers WHERE id = $1;
