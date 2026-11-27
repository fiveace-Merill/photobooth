# Schema notes

Read this before changing anything in `/migrations` or writing queries. The migrations are the source of truth; this explains the intent.

## Tables by slice

| Migration | Tables | Purpose |
|---|---|---|
| 00001 | `plans`, `photographers`, `sessions` | Tenants, plan config (placeholder prices), photographer login sessions |
| 00002 | `clients`, `shoots` | Client catalogue; shoots with status and due date |
| 00003 | `photos`, `gallery_links` | R2 object keys; magic-link access for clients |
| 00004 | `payments`, `payment_callbacks`, `payouts`, `ledger_entries` | Money. Apply when reaching the payments slice |
| 00005 | `shoot_events`, photo processing fields | Photo processing state/audit events |
| 00006 | photo processing recovery function | Reaps stale processing photos |
| 00007 | `notifications` | Delivery email notification records (no body/URL/token) |

## Rules the schema relies on

1. **Tenant scoping.** Every tenant table has `photographer_id`. Composite foreign keys `(photographer_id, parent_id)` stop cross-tenant links. RLS is a second net behind explicit `WHERE photographer_id = ...` in queries.
2. **No tenant context means no rows.** If `app.photographer_id` is unset, RLS matches nothing. An empty result where you expected data usually means the helper was not called.
3. **Money** is `bigint` in the smallest unit (KES cents). Currency is stored alongside.
4. **Ledger is append-only.** A trigger rejects UPDATE and DELETE. Entries sharing a `txn_id` must sum to zero; enforce this in Go and test it. Reverse with new entries (`reverses_txn_id`), never edits.
5. **Idempotent payments.** `payments.provider_txn_id` is unique per provider. A repeated M-PESA callback must be a no-op. Save the raw callback in `payment_callbacks` first.
6. **Server-only unlock.** `shoots.originals_unlocked_at` is set only by server code after a verified payment.
7. **Rates are snapshotted.** `payments.commission_bps` / `commission_minor` record what applied at payment time. Do not recompute history from the current plan.
8. **Secrets are stored hashed.** `sessions.token_hash`, `gallery_links.token_hash`, `access_code_hash`. Never store raw tokens.

## Placeholder values (to be replaced after market research)

Seeded in `plans`: Free = 0/month, 8% commission, 5 GB, 30 days. Pro = KES 1,000/month, 3% commission, 100 GB, 180 days. Change them with an `UPDATE`/new migration, not in Go code.

## Shoot status semantics

- `shot`, `editing`, `ready`, `delivered`, `expired` are the stored statuses.
- Photographers may set `shot`, `editing`, `ready` through the API, in any order. `delivered` and `expired` are set only by system code in later slices.
- `price_minor = 0` means free delivery.

## Not designed yet

Notifications log, reviews/testimonials, expiry job bookkeeping, subscription billing for photographers. Design each in its own slice.

## Photos / storage (slice 3a)

- `photos` gained `error`, `processed_at`, `sort_key`, `updated_at`, `upload_id` in 00005. Originals live in R2; previews/thumbs are generated derivatives. Original keys/URLs must never be exposed to clients before unlock.
- Object keys are ID-only: `p/{photographerId}/{shootId}/{photoId}/{original|preview.jpg|thumb.jpg}`. Never include user text.
- Upload flow reserves quota atomically (`photographers.storage_used_bytes` checked against plan quota in one UPDATE). Completion verifies object HEAD size equals declared size; mismatch deletes object, releases quota, marks photo failed `size_mismatch`.
- `shoot_events` is an internal audit log (RLS same tenant policy) for upload/processing/delete events.

## notifications (slice 4)

`notifications` records delivery email attempts/outcomes. It intentionally stores **no message body and no delivery URL/token**.

- Columns: `photographer_id, shoot_id, client_id, channel('email'), kind('delivery_link'), recipient, status('queued'|'sent'|'failed'), provider_message_id, error_code, created_at, sent_at`.
- Tenant-scoped with composite FKs to `shoots` and `clients`, and RLS copied from tenant tables.
- `queued` rows are written before SMTP send and updated after the inline send completes.
- A small periodic River job marks rows stuck in `queued` for >10 minutes as `failed/send_timeout`.
- Delivery links are sent inline in the create/regenerate request; no background job carries a raw URL/token.
