-- Queries for the clients package. sqlc turns these into Go in internal/clients/store.
-- RULE: every query on tenant data filters by photographer_id.

-- name: ListClients :many
SELECT id, photographer_id, name, phone, email, notes, created_at, updated_at
FROM clients
WHERE photographer_id = $1
ORDER BY name;

-- name: GetClient :one
SELECT id, photographer_id, name, phone, email, notes, created_at, updated_at
FROM clients
WHERE photographer_id = $1 AND id = $2;

-- name: CreateClient :one
INSERT INTO clients (photographer_id, name, phone, email, notes)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, photographer_id, name, phone, email, notes, created_at, updated_at;
