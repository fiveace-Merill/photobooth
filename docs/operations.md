# Operations

## Create the first admin

1. Ensure `.env` is configured (including `ADMIN_TOTP_ENC_KEY`, 32-byte hex: `openssl rand -hex 32`) and migrations are applied (`make migrate-up`).
2. Create an admin with the CLI (password is read with no echo, never passed as a flag/env var):

```bash
make admin ARGS="admin create --email admin@example.com"
```

3. Log in via `POST /api/v1/admin/auth/login`. If 2FA is required, complete setup:
   - `POST /api/v1/admin/auth/totp/setup/start` (pending admin session) -> returns `otpauthUri` and `secret` once.
   - Add the secret/URI to an authenticator app.
   - `POST /api/v1/admin/auth/totp/setup/confirm` with `{code}` -> returns 10 recovery codes once. Store them securely; only hashes are stored server-side.

## Invite a photographer from the waitlist

1. Review waitlist entries in admin API: `GET /api/v1/admin/waitlist?status=new`.
2. Invite: `POST /api/v1/admin/waitlist/{id}/invite` with `{email?, planId, days, send}`.
   - If the entry is phone-only, provide `email`; it will be saved on the waitlist entry.
   - Response includes `url` (invite link, shown once), `expiresAt`, and `emailStatus`.
   - If `send=true` and email sender is configured, Photobooth sends the invitation inline. The raw URL is never logged or stored.
3. Photographer signs up via `/signup?invite=...` (invite mode). Invitation is single-use and bound to email+plan.

## Handle an erasure request

For a waitlist/contact erasure request:

1. Confirm identity through your support process.
2. Export/audit as needed for internal records before deletion (exports are audited).
3. Delete the waitlist entry: `DELETE /api/v1/admin/waitlist/{id}`. This hard-deletes the row.
4. Do not include raw contact details in audit meta; audit entries remain append-only.

For photographer/account erasure requests, follow legal/compliance process; this admin slice does not delete photographer tenant content.

## If an admin loses their authenticator

1. Use a recovery code at `POST /api/v1/admin/auth/totp/verify` with `{recoveryCode}`. Recovery codes are single-use.
2. If no recovery codes remain, a trusted operator with CLI/DB access resets 2FA:

```bash
make admin ARGS="admin reset-2fa --email admin@example.com"
```

This clears the TOTP secret and recovery codes and writes a CLI audit row. The admin must re-enrol at next login.

## Trusted proxies and client IP

`TRUSTED_PROXY_CIDRS` defaults to empty, which means the API uses `RemoteAddr` only and ignores `X-Forwarded-For` completely. This is safe when the API is directly reachable.

Only set `TRUSTED_PROXY_CIDRS` when the API is behind a trusted reverse proxy that **overwrites** `X-Forwarded-For` (not appends untrusted client input blindly). When the direct peer is in `TRUSTED_PROXY_CIDRS`, the API walks `X-Forwarded-For` right-to-left and uses the rightmost entry that is not itself a trusted proxy. Malformed/empty forwarded headers fall back to `RemoteAddr`.

For Caddy, the required config is to ensure Caddy sets/overwrites forwarding headers for requests it proxies, and only the Caddy address/network is listed in `TRUSTED_PROXY_CIDRS`. Example (adapt to your deployment):

```caddyfile
photobooth.example.com {
    reverse_proxy 127.0.0.1:8080
}
```

Caddy replaces forwarding headers by default for proxied requests; do not add custom handlers that copy arbitrary client-supplied `X-Forwarded-For` values. Set `TRUSTED_PROXY_CIDRS=127.0.0.1/32,::1/128` (or your proxy network) only when Caddy is the only direct peer allowed to reach the API. If clients can reach the API directly, leave `TRUSTED_PROXY_CIDRS` empty.
