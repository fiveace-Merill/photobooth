-- name: GetShootForPhoto :one
SELECT s.id, s.photographer_id, s.client_id, s.status, c.name AS client_name
FROM shoots s
JOIN clients c ON c.photographer_id = s.photographer_id AND c.id = s.client_id
WHERE s.photographer_id = $1 AND s.id = $2;

-- name: CountActivePhotosByShoot :one
SELECT COUNT(*)::int AS count
FROM photos
WHERE photographer_id = $1 AND shoot_id = $2 AND deleted_at IS NULL;

-- name: ReservePhotoQuota :one
UPDATE photographers p
SET storage_used_bytes = p.storage_used_bytes + $2
FROM plans pl
WHERE p.id = $1
  AND p.plan_id = pl.id
  AND p.storage_used_bytes + $2 <= pl.storage_quota_bytes
RETURNING p.storage_used_bytes;

-- name: ReleasePhotoQuota :exec
UPDATE photographers
SET storage_used_bytes = GREATEST(storage_used_bytes - $2, 0)
WHERE id = $1;

-- name: GetPhotographerQuota :one
SELECT p.storage_used_bytes, pl.storage_quota_bytes
FROM photographers p
JOIN plans pl ON pl.id = p.plan_id
WHERE p.id = $1;

-- name: CreatePhoto :one
INSERT INTO photos (
  id, photographer_id, shoot_id, filename, content_type, size_bytes,
  original_key, preview_key, thumb_key, status, sort_key, upload_id
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
RETURNING id, photographer_id, shoot_id, filename, content_type, size_bytes, width, height,
          status, error, processed_at, sort_key, created_at, updated_at, upload_id;

-- name: GetPhoto :one
SELECT id, photographer_id, shoot_id, filename, content_type, size_bytes, width, height,
       original_key, preview_key, thumb_key, status, error, processed_at, sort_key, created_at, updated_at, upload_id
FROM photos
WHERE photographer_id = $1 AND shoot_id = $2 AND id = $3 AND deleted_at IS NULL;

-- name: MarkPhotoProcessing :one
UPDATE photos
SET status = 'processing', error = NULL, updated_at = now()
WHERE photographer_id = $1 AND shoot_id = $2 AND id = $3 AND deleted_at IS NULL
RETURNING id, photographer_id, shoot_id, filename, content_type, size_bytes, width, height,
          status, error, processed_at, sort_key, created_at, updated_at, upload_id;

-- name: MarkPhotoFailed :one
UPDATE photos
SET status = 'failed', error = $4, updated_at = now()
WHERE photographer_id = $1 AND shoot_id = $2 AND id = $3 AND deleted_at IS NULL
RETURNING id, photographer_id, shoot_id, filename, content_type, size_bytes, width, height,
          status, error, processed_at, sort_key, created_at, updated_at, upload_id;

-- name: MarkPhotoReady :one
UPDATE photos
SET status = 'ready', error = NULL, width = $4, height = $5, processed_at = now(), updated_at = now()
WHERE photographer_id = $1 AND shoot_id = $2 AND id = $3 AND deleted_at IS NULL
RETURNING id, photographer_id, shoot_id, filename, content_type, size_bytes, width, height,
          status, error, processed_at, sort_key, created_at, updated_at, upload_id;

-- name: DeletePhotoReturning :one
DELETE FROM photos
WHERE photographer_id = $1 AND shoot_id = $2 AND id = $3
RETURNING id, photographer_id, shoot_id, size_bytes, original_key, preview_key, thumb_key, status, upload_id;

-- name: ListPhotosByShoot :many
SELECT id, photographer_id, shoot_id, filename, content_type, size_bytes, width, height,
       status, error, processed_at, sort_key, created_at, updated_at, thumb_key, preview_key
FROM photos
WHERE photographer_id = $1 AND shoot_id = $2 AND deleted_at IS NULL
ORDER BY sort_key ASC, created_at ASC, id ASC
LIMIT $3 OFFSET $4;

-- name: CountPhotosByShoot :one
SELECT COUNT(*)::bigint AS total, COALESCE(SUM(size_bytes),0)::bigint AS bytes
FROM photos
WHERE photographer_id = $1 AND shoot_id = $2 AND deleted_at IS NULL;

-- name: CreateShootEvent :exec
INSERT INTO shoot_events (photographer_id, shoot_id, photo_id, kind, payload)
VALUES ($1, $2, $3, $4, $5);
