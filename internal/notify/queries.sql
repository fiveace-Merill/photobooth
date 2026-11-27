-- name: InsertNotificationQueued :one
INSERT INTO notifications (
    photographer_id, shoot_id, client_id, channel, kind, recipient, status
) VALUES ($1, $2, $3, 'email', 'delivery_link', $4, 'queued')
RETURNING id, photographer_id, shoot_id, client_id, channel, kind, recipient, status, provider_message_id, error_code, created_at, sent_at;

-- name: MarkNotificationSent :exec
UPDATE notifications
SET status = 'sent', provider_message_id = $2, error_code = NULL, sent_at = now()
WHERE id = $1 AND status = 'queued';

-- name: MarkNotificationFailed :exec
UPDATE notifications
SET status = 'failed', error_code = $2, sent_at = NULL
WHERE id = $1 AND status = 'queued';

-- name: ListShootNotifications :many
SELECT id, photographer_id, shoot_id, client_id, channel, kind, recipient, status, provider_message_id, error_code, created_at, sent_at
FROM notifications
WHERE photographer_id = $1 AND shoot_id = $2
ORDER BY created_at DESC, id DESC
LIMIT $3;

-- name: CountShootEmailAttemptsSince :one
SELECT COUNT(*)::int
FROM notifications
WHERE photographer_id = $1 AND shoot_id = $2
  AND channel = 'email' AND kind = 'delivery_link'
  AND created_at >= $3;

-- name: CountPhotographerEmailAttemptsSince :one
SELECT COUNT(*)::int
FROM notifications
WHERE photographer_id = $1
  AND channel = 'email' AND kind = 'delivery_link'
  AND created_at >= $2;

-- name: FailStaleQueuedNotifications :execrows
UPDATE notifications
SET status = 'failed', error_code = 'send_timeout', sent_at = NULL
WHERE status = 'queued' AND created_at < now() - interval '10 minutes';
