# API

All endpoints are under `/api/v1`. JSON uses camelCase. Money is integer minor units (KES cents). Dates are `YYYY-MM-DD`. Timestamps are RFC3339 UTC.

Authentication: session cookie `photobooth_session`, set by auth endpoints. State-changing requests require `Content-Type: application/json` and same-origin `Origin` when present.

Error shape:

```json
{ "error": "human readable", "code": "stable_code" }
```

Some errors include extra fields (documented below).

## Auth

- `POST /api/v1/auth/signup` - `{email,password,business_name,slug}`. 201 auth response; 403 `signup_closed`; 409 `email_taken`/`slug_taken`.
- `POST /api/v1/auth/login` - `{email,password}`. 200 auth response; 401 `invalid_credentials`.
- `POST /api/v1/auth/logout` - 204.
- `GET /api/v1/auth/me` - 200 `{id,email,business_name,slug,plan_id}`.

## Clients

All client endpoints require authentication. A resource owned by another photographer returns 404.

### GET /api/v1/clients?q=&limit=&offset=
Search by name, phone, email (case-insensitive). Ordered by name. Default limit 25, max 100.

Response `200`:
```json
{ "items": [Client], "total": 1, "limit": 25, "offset": 0 }
```

### POST /api/v1/clients
Body: `{name, phone?, email?, notes?}`. At least one of phone/email required. Phone is normalized before storage.

Response `201`: Client.

Errors: `400 invalid_input`, `409 client_phone_exists` with `existingClientId`.

### GET /api/v1/clients/{id}
Response `200`: Client. `404 not_found`.

### PATCH /api/v1/clients/{id}
Partial update. Response `200`: Client. Errors: `400`, `404`, `409 client_phone_exists`.

### DELETE /api/v1/clients/{id}
Response `204`. Errors: `404 not_found`, `409 client_has_shoots`.

Client shape:
```json
{ "id":"uuid","name":"...","phone":"2547...","email":"...","notes":"...","createdAt":"RFC3339","updatedAt":"RFC3339" }
```

## Shoots

All shoot endpoints require authentication. A resource owned by another photographer returns 404.

### GET /api/v1/shoots?status=&overdue=&clientId=&q=&sort=&limit=&offset=
- `status`: comma list of `shot,editing,ready,delivered,expired`
- `overdue`: `true|false` (overdue = today in Africa/Nairobi after dueDate and status shot/editing/ready; due today is not overdue)
- `clientId`: UUID
- `q`: matches title
- `sort`: `due` (default, due date asc, nulls last, newest) or `created`
- paging: default limit 25, max 100

Response `200`:
```json
{ "items":[Shoot], "total": 1, "limit": 25, "offset": 0 }
```

### POST /api/v1/shoots
Body: `{clientId,title,shootDate?,dueDate?,priceMinor?,currency?}`. Currency fixed to `KES`. `priceMinor` 0-100000000. `priceMinor=0` means free delivery.

Response `201`: Shoot. Errors: `400 invalid_input`, `404 client_not_found`.

### GET /api/v1/shoots/{id}
Response `200`: Shoot.

### PATCH /api/v1/shoots/{id}
Partial update for clientId/title/shootDate/dueDate/priceMinor/currency. Status changes use `/status`.

### POST /api/v1/shoots/{id}/status
Body: `{status}`. Photographers may set `shot`, `editing`, `ready` in any order. `delivered` and `expired` return `422 invalid_status_transition`.

### DELETE /api/v1/shoots/{id}
Only `shot,editing,ready` shoots can be deleted. Response `204`; `409 shoot_not_deletable`.

### GET /api/v1/shoots/summary
Response `200`:
```json
{ "counts": {"shot":0,"editing":0,"ready":0,"delivered":0,"expired":0}, "overdue": 0 }
```

Shoot shape:
```json
{
  "id":"uuid","clientId":"uuid","clientName":"...","title":"...",
  "shootDate":"YYYY-MM-DD","status":"shot","dueDate":"YYYY-MM-DD",
  "priceMinor":300000,"currency":"KES","overdue":false,"daysOverdue":0,
  "createdAt":"RFC3339","updatedAt":"RFC3339"
}
```

## Photos (photographer)

All endpoints require session auth. Any shoot/photo not owned by the session photographer returns `404 not_found`. Never expose original URLs/keys before unlock; storage keys contain no user text.

### POST /api/v1/shoots/{shootId}/photos/uploads
Body: `{filename, contentType, sizeBytes}`. Allowed types: `image/jpeg,image/png,image/webp`. Size `1..MAX_PHOTO_BYTES`. Reserves quota atomically and initiates multipart upload.

Response `201`:
```json
{ "photoId":"uuid","shootId":"uuid","partSizeBytes":10485760,"partCount":3,"uploadParts":[{"partNumber":1,"url":"...","expiresAt":"RFC3339"}],"status":"uploading" }
```

Errors: `415 unsupported_type`, `413 file_too_large`, `409 quota_exceeded`, `409 too_many_photos`, `409 shoot_not_uploadable`.

### POST /api/v1/shoots/{shootId}/photos/{photoId}/uploads/parts
Body: `{partNumbers:[4,5,...]}` (max 20). Returns presigned upload part URLs.

### POST /api/v1/shoots/{shootId}/photos/{photoId}/complete
Body: `{parts:[{"partNumber":1,"etag":"..."}]}`. Completes multipart, verifies HEAD size equals declared size; mismatch deletes object, releases quota, marks failed `422 size_mismatch`. On success marks `processing` and enqueues `process_photo` in same River transaction.

### GET /api/v1/shoots/{shootId}/photos?limit=&offset=
Response `200`:
```json
{ "items":[Photo],"totals":{"count":0,"bytes":0},"quota":{"usedBytes":0,"quotaBytes":0} }
```
Photo includes `thumbUrl` only when ready; `thumbUrl` is a short-lived presigned GET. No original URL/key.

### POST /api/v1/shoots/{shootId}/photos/{photoId}/retry
Retries failed photo processing. Only `failed` photos: `409 invalid_photo_state`.

### DELETE /api/v1/shoots/{shootId}/photos/{photoId}
Aborts upload if uploading; deletes DB row and releases quota exactly once; best-effort deletes objects. Response `204`.

## Stable error codes

| Code | Status | Meaning |
|---|---:|---|
| `invalid_json` | 400 | Malformed/invalid JSON body |
| `invalid_input` | 400 | Validation failed |
| `unauthorized` | 401 | Missing/invalid session |
| `not_found` | 404 | Resource missing or owned by another photographer |
| `client_not_found` | 404 | Shoot client missing or not owned |
| `client_phone_exists` | 409 | Duplicate phone for photographer; includes `existingClientId` |
| `client_has_shoots` | 409 | Client cannot be deleted because shoots reference it |
| `shoot_not_deletable` | 409 | Shoot status disallows deletion or FK blocks it |
| `invalid_status_transition` | 422 | Status not allowed for photographer/API |
| `invalid_content_type` | 415 | State-changing request missing JSON content type |
| `invalid_origin` | 403 | Cross-site Origin rejected |
| `signup_closed` | 403 | Public signup disabled |
| `internal` | 500 | Unexpected server error |
| `unsupported_type` | 415 | Photo content type not jpeg/png/webp |
| `file_too_large` | 413 | Photo exceeds MAX_PHOTO_BYTES |
| `quota_exceeded` | 409 | Photographer storage quota would be exceeded |
| `too_many_photos` | 409 | Shoot exceeds MAX_PHOTOS_PER_SHOOT |
| `shoot_not_uploadable` | 409 | Shoot cannot accept uploads (e.g. expired) |
| `invalid_photo_state` | 409 | Photo state does not allow operation |
| `size_mismatch` | 422 | Completed object size differs from declared size |
| `already_delivered` | 409 | Shoot already has an active delivery link |
| `shoot_not_deliverable` | 409 | Shoot has no ready photos or still has processing photos |
| `not_delivered` | 409 | Shoot has not been delivered yet |
| `no_active_link` | 409 | No active delivery link to revoke |
| `already_unlocked` | 409 | Originals already unlocked |
| `invalid_mark_paid` | 409 | Manual mark-paid not allowed in current state |
| `originals_locked` | 403 | Public original/download requested before unlock |
| `rate_limited` | 429 | Public delivery IP rate limit exceeded |
| `channel_unavailable` | 422 | Requested notification channel unavailable (e.g. client missing/invalid email) |
| `too_many_sends` | 429 | Too many delivery email attempts for this shoot (5/hour) |
| `email_daily_limit_reached` | 429 | Photographer daily email limit reached (Africa/Nairobi day) |

## Delivery (photographer, session)

All endpoints require authentication and return 404 for shoots not owned by the session photographer.

Delivery links are sent inline in the create/regenerate request when requested. The raw delivery token/URL is never persisted or logged; it is only returned in the create/regenerate response and, if email send is requested, sent directly to the mail server in the same request.

### POST /api/v1/shoots/{id}/deliver
Creates a private gallery link and marks shoot as delivered. Requires at least one ready photo and no processing photos.

Optional body:
```json
{ "send": { "email": true } }
```

Rules:
- If `priceMinor == 0`, originals unlock at delivery (`originals_unlocked_at` set).
- If `priceMinor > 0`, originals remain locked until mark-paid (pilot) or verified payment (M-PESA slice).
- `delivered` is a system action; photographers cannot set it via `/status`.
- if `send.email` is true and client has no valid email: `422 channel_unavailable` before any link is created
- if `send.email` is true, per-shoot limit is 5 send attempts/hour: `429 too_many_sends`; daily per-photographer cap is `EMAIL_DAILY_LIMIT_PER_PHOTOGRAPHER` counted in Africa/Nairobi days: `429 email_daily_limit_reached`
- link creation is committed before email send; provider failure never rolls back link creation
- email send result is reported in `notifications`; "sent" means the mail server accepted the message, never "delivered" to inbox

Response `201`:
```json
{
  "url": "https://app.example.com/g/<token>",
  "expiresAt": "RFC3339",
  "notifications": [
    { "channel": "email", "status": "sent|failed", "errorCode": "recipient_rejected|send_failed|send_timeout" }
  ]
}
```
`notifications` is empty/omitted when no send was requested. The raw token is shown once and never stored or returned again. Errors: `409 already_delivered`, `409 shoot_not_deliverable`.

### POST /api/v1/shoots/{id}/link/regenerate
Revokes active link, creates a new link, writes shoot event. "Send again" uses this endpoint with `{ "send": { "email": true } }`; regenerating invalidates the previous link.

- not delivered yet: `409 not_delivered`
- same send/limit/channel rules as deliver

Response `200`: same shape as deliver.

### POST /api/v1/shoots/{id}/link/revoke
Revokes active link and writes shoot event. Response `204`. 409 `no_active_link`.

### POST /api/v1/shoots/{id}/mark-paid
Manual pilot unlock for payments received outside Photobooth. Body: `{method: "cash"|"mpesa_direct"|"other", note?}`.

Allowed only when shoot is delivered, `priceMinor > 0`, and originals are not unlocked. Sets `originals_unlocked_at`, writes `manual_mark_paid` event. Does **not** write `payments` or `ledger_entries`; manual payments sit outside the commission system.

Response `204`. Errors: `409 already_unlocked`, `409 invalid_mark_paid`, `400 invalid_input`.

### GET /api/v1/shoots/{id}/delivery
Response `200`:
```json
{
  "linkState": "active|revoked|none",
  "expiresAt": "RFC3339",
  "lastViewedAt": "RFC3339",
  "unlocked": false,
  "events": [{"id":"uuid","shootId":"uuid","photoId":"uuid|null","kind":"...","payload":{},"createdAt":"RFC3339"}]
}
```
Never returns token or URL. `events` contains last 20 shoot events.

### GET /api/v1/shoots/{id}/notifications
Lists delivery notification attempts for a shoot. Recipient is masked (`j***@gmail.com`).

Response `200`:
```json
{
  "items": [
    {
      "channel": "email",
      "status": "queued|sent|failed",
      "createdAt": "RFC3339",
      "errorCode": "send_failed",
      "recipient": "j***@gmail.com"
    }
  ]
}
```

### GET /api/v1/shoots/{id}/delivery/message-preview
Returns a safe email preview with placeholder URL (never a real delivery token). Auth required.

Response `200`:
```json
{
  "subject": "Your photos from Wedding are ready",
  "bodyText": "... https://photobooth.example/g/preview ...",
  "recipientMasked": "j***@gmail.com",
  "emailsRemainingToday": 49,
  "availability": {
    "email": { "available": true, "reason": "" }
  }
}
```

## Delivery (public, no session)

Public endpoints are rate limited per IP with an in-memory single-process limiter (resets on restart). Responses include `Cache-Control: private, no-store` and `Referrer-Policy: no-referrer`.

Invalid, revoked, and expired tokens return identical `404` with body `{"error":"Not found","code":"not_found"}`.

### GET /api/v1/deliveries/{token}
Returns delivery DTO. Updates `last_viewed_at` at most once per minute.

Preview DTO (locked) - never contains original URL/key:
```json
{
  "status":"preview",
  "studioName":"Studio",
  "studioPhone":"2547...",
  "title":"Shoot",
  "shotOn":"YYYY-MM-DD",
  "priceMinor":300000,
  "currency":"KES",
  "expiresAt":"RFC3339",
  "features":{"payments":"manual","reviews":false},
  "photos":[{"id":"uuid","width":1200,"height":800,"thumbUrl":"...","previewUrl":"..."}]
}
```

Unlocked DTO adds original API paths and total bytes:
```json
{
  "status":"unlocked",
  "...":"...",
  "downloadAllUrl":"/api/v1/deliveries/{token}/download",
  "totalOriginalBytes":12345,
  "photos":[{"...":"...","originalUrl":"/api/v1/deliveries/{token}/photos/{photoId}/original"}]
}
```

`thumbUrl` and `previewUrl` are presigned R2 GET URLs (TTL 1h) to watermarked files.

### GET /api/v1/deliveries/{token}/photos/{photoId}/original
Checks unlock on every request. `403 originals_locked` when locked. When unlocked, returns `302` to presigned R2 GET (TTL 15m) with `Content-Disposition` attachment filename (RFC 5987).

### GET /api/v1/deliveries/{token}/download
Unlocked only. Streams a ZIP (`application/zip`) with `Store` (no compression) directly from R2 without buffering whole files/ZIP. Filename is sanitized shoot title. Logs `delivery_download_all` event.

Limits/honesty: no HTTP range/resume; if connection drops client restarts. Long request: reverse proxy (Caddy) must allow long read/write/idle timeouts; the API sets per-request write deadline for downloads and does not set a global `WriteTimeout`. Streams through VPS bandwidth; large shoots may take minutes. ZIP size/content length is not known in advance.
