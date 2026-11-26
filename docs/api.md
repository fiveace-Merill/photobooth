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
