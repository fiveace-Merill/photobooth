-- RULE: every query on tenant data filters by photographer_id.

-- name: CreateShoot :one
INSERT INTO shoots (photographer_id, client_id, title, shoot_date, status, due_date, price_minor, currency)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING id, photographer_id, client_id, title, shoot_date, status, due_date, price_minor, currency, originals_unlocked_at, delivered_at, created_at, updated_at;

-- name: GetShoot :one
SELECT id, photographer_id, client_id, title, shoot_date, status, due_date, price_minor, currency, originals_unlocked_at, delivered_at, created_at, updated_at
FROM shoots
WHERE photographer_id = $1 AND id = $2;

-- name: UpdateShoot :one
UPDATE shoots
SET client_id = $3,
    title = $4,
    shoot_date = $5,
    due_date = $6,
    price_minor = $7,
    currency = $8,
    updated_at = now()
WHERE photographer_id = $1 AND id = $2
RETURNING id, photographer_id, client_id, title, shoot_date, status, due_date, price_minor, currency, originals_unlocked_at, delivered_at, created_at, updated_at;

-- name: UpdateShootStatus :one
UPDATE shoots
SET status = $3, updated_at = now()
WHERE photographer_id = $1 AND id = $2
RETURNING id, photographer_id, client_id, title, shoot_date, status, due_date, price_minor, currency, originals_unlocked_at, delivered_at, created_at, updated_at;

-- name: DeleteShoot :execrows
DELETE FROM shoots
WHERE photographer_id = $1 AND id = $2;

-- name: CountShoots :one
SELECT COUNT(*)
FROM shoots
WHERE photographer_id = $1
  AND (sqlc.narg('status_csv')::text IS NULL OR status = ANY(string_to_array(sqlc.narg('status_csv')::text, ',')))
  AND (sqlc.narg('client_id')::uuid IS NULL OR client_id = sqlc.narg('client_id')::uuid)
  AND (sqlc.narg('q')::text IS NULL OR title ILIKE '%' || sqlc.narg('q')::text || '%')
  AND (
    sqlc.narg('overdue')::bool IS NULL
    OR (
      sqlc.narg('overdue')::bool = true AND due_date < sqlc.arg('today')::date AND status IN ('shot','editing','ready')
    )
    OR (
      sqlc.narg('overdue')::bool = false AND NOT (due_date < sqlc.arg('today')::date AND status IN ('shot','editing','ready'))
    )
  );

-- name: ListShootsByDue :many
SELECT id, photographer_id, client_id, title, shoot_date, status, due_date, price_minor, currency, originals_unlocked_at, delivered_at, created_at, updated_at
FROM shoots
WHERE photographer_id = $1
  AND (sqlc.narg('status_csv')::text IS NULL OR status = ANY(string_to_array(sqlc.narg('status_csv')::text, ',')))
  AND (sqlc.narg('client_id')::uuid IS NULL OR client_id = sqlc.narg('client_id')::uuid)
  AND (sqlc.narg('q')::text IS NULL OR title ILIKE '%' || sqlc.narg('q')::text || '%')
  AND (
    sqlc.narg('overdue')::bool IS NULL
    OR (
      sqlc.narg('overdue')::bool = true AND due_date < sqlc.arg('today')::date AND status IN ('shot','editing','ready')
    )
    OR (
      sqlc.narg('overdue')::bool = false AND NOT (due_date < sqlc.arg('today')::date AND status IN ('shot','editing','ready'))
    )
  )
ORDER BY due_date ASC NULLS LAST, created_at DESC
LIMIT $2 OFFSET $3;

-- name: ListShootsByCreated :many
SELECT id, photographer_id, client_id, title, shoot_date, status, due_date, price_minor, currency, originals_unlocked_at, delivered_at, created_at, updated_at
FROM shoots
WHERE photographer_id = $1
  AND (sqlc.narg('status_csv')::text IS NULL OR status = ANY(string_to_array(sqlc.narg('status_csv')::text, ',')))
  AND (sqlc.narg('client_id')::uuid IS NULL OR client_id = sqlc.narg('client_id')::uuid)
  AND (sqlc.narg('q')::text IS NULL OR title ILIKE '%' || sqlc.narg('q')::text || '%')
  AND (
    sqlc.narg('overdue')::bool IS NULL
    OR (
      sqlc.narg('overdue')::bool = true AND due_date < sqlc.arg('today')::date AND status IN ('shot','editing','ready')
    )
    OR (
      sqlc.narg('overdue')::bool = false AND NOT (due_date < sqlc.arg('today')::date AND status IN ('shot','editing','ready'))
    )
  )
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: ShootSummary :one
SELECT
  COUNT(*) FILTER (WHERE status = 'shot')::bigint AS shot,
  COUNT(*) FILTER (WHERE status = 'editing')::bigint AS editing,
  COUNT(*) FILTER (WHERE status = 'ready')::bigint AS ready,
  COUNT(*) FILTER (WHERE status = 'delivered')::bigint AS delivered,
  COUNT(*) FILTER (WHERE status = 'expired')::bigint AS expired,
  COUNT(*) FILTER (WHERE due_date < $2::date AND status IN ('shot','editing','ready'))::bigint AS overdue
FROM shoots
WHERE photographer_id = $1;

-- name: ListPhotoKeysByShoot :many
SELECT original_key, preview_key, thumb_key
FROM photos
WHERE photographer_id = $1 AND shoot_id = $2;
