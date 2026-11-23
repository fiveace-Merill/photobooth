# Schema notes

Read this before changing anything in `/migrations` or writing queries. The migrations are the source of truth; this explains the intent.

## Tables by slice

| Migration | Tables | Purpose |
|---|---|---|
| 00001 | `plans`, `photographers`, `sessions` | Tenants, plan config (placeholder prices), photographer login sessions |
| 00002 | `clients`, `shoots` | Client catalogue; shoots with status and due date |
| 00003 | `photos`, `gallery_links` | R2 object keys; magic-link access for clients |
| 00004 | `payments`, `payment_callbacks`, `payouts`, `ledger_entries` | Money. Apply when reaching the payments slice |

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

## Not designed yet

Notifications log, reviews/testimonials, expiry job bookkeeping, subscription billing for photographers. Design each in its own slice.
