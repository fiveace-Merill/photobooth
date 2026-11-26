-- Queries for the clients package. sqlc turns these into Go in internal/clients/store.
-- RULE: every query on tenant data filters by photographer_id.

-- name: ListClients :many
SELECT id, photographer_id, name, phone, email, notes, created_at, updated_at,
       COUNT(*) OVER() AS total
FROM clients
WHERE photographer_id = $1
  AND (
    sqlc.narg('q')::text IS NULL
    OR name ILIKE '%' || sqlc.narg('q')::text || '%'
    OR COALESCE(phone, '') ILIKE '%' || sqlc.narg('q')::text || '%'
    OR COALESCE(email::text, '') ILIKE '%' || sqlc.narg('q')::text || '%'
  )
ORDER BY name
LIMIT $2 OFFSET $3;

-- name: GetClient :one
SELECT id, photographer_id, name, phone, email, notes, created_at, updated_at
FROM clients
WHERE photographer_id = $1 AND id = $2;

-- name: CreateClient :one
INSERT INTO clients (photographer_id, name, phone, email, notes)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, photographer_id, name, phone, email, notes, created_at, updated_at;

-- name: UpdateClient :one
UPDATE clients
SET name = $3,
    phone = $4,
    email = $5,
    notes = $6,
    updated_at = now()
WHERE photographer_id = $1 AND id = $2
RETURNING id, photographer_id, name, phone, email, notes, created_at, updated_at;

-- name: DeleteClient :execrows
DELETE FROM clients
WHERE photographer_id = $1 AND id = $2;

-- name: ClientExists :one
SELECT EXISTS(
  SELECT 1 FROM clients WHERE photographer_id = $1 AND id = $2
) AS exists;

-- name: GetClientByPhone :one
SELECT id, photographer_id, name, phone, email, notes, created_at, updated_at
FROM clients
WHERE photographer_id = $1 AND phone = $2;

-- name: NamesByIDs :many
SELECT id, name
FROM clients
WHERE photographer_id = $1 AND id = ANY(sqlc.arg('ids')::uuid[]);
