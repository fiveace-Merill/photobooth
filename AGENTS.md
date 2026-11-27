# AGENTS.md

Read fully before changing code. When prose and code/config disagree, trust the executable source.

- The client delivery page is built against the DeliveryApi interface in web/lib/api/client.ts, currently implemented by mock.ts. The Go API must return data matching web/lib/api/types.ts. Changing that contract means changing both sides.

## Product & current state
Photobooth delivers photos for Kenyan photographers: photographers are paying tenants (email/password); their clients never sign up — they open magic links, view watermarked previews, approve, pay with M-PESA, then originals unlock. Revenue is commission per paid delivery plus optional `plans` (placeholder values in the DB, expected to change).

- Only `GET /healthz` exists today; `cmd/api/main.go` is wiring only.
- `internal/clients/` is the only real domain package (`queries.sql` + generated `store/`). Other `internal/*/doc.go` files are marked "Not started" — implement one slice at a time; do not assume code exists.
- Migrations 00001–00004 are written, including payments designed ahead. `web/` is not scaffolded (only `theme.css` and `README.md`).
- `go.mod` pins **Go 1.25.0**; older "1.22+" mentions in docs are stale.

## Boundaries
Go stdlib `net/http`, Postgres with pgx/sqlc/goose and row-level security. Planned: River jobs, Cloudflare R2 + image previews, Next.js in `web/`. Do not add Prisma/any ORM, Redis, GraphQL, microservices or a new framework without asking; justify new dependencies against a stdlib alternative.

- Never import another domain's `store/`; call its exported service interface. `internal/platform/` (config, database, httpx, testdb) is the only code every package may import, and holds no business rules.
- Domain package shape: `doc.go`, `handler.go`, `service.go`, `queries.sql`, `store/` (sqlc-generated — never hand-edit), `*_test.go`. Each package owns its tables and queries.
- A package's first `queries.sql` needs a matching block in `sqlc.yaml` (only `clients` is active), then `make sqlc`.
- No `pkg/`, `utils/`, `helpers/`, `models/` or `controllers/` folders. The frontend calls the Go API only — no business logic or DB access in `web/`.
- `migrations/` is the schema source of truth; `db/init/01_roles.sh` creates the non-owner `photobooth_app` role once, on a fresh Docker volume.

## Commands
`make help` lists all targets. The Makefile does `-include .env` + `export`, so targets already see `.env`; do not source it manually.

```
cp .env.example .env          # first time; change the passwords
make db-up                    # Postgres 16 in Docker, waits on healthcheck
make migrate-up               # as owner (MIGRATE_DATABASE_URL)
make sqlc                     # after ANY queries.sql edit
make run                      # go run ./cmd/api; PORT overrides 8080
make db-reset                 # docker compose down -v, fresh volume
make migrate-new name=x       # new goose migration
make check                    # gofmt -w + go vet + go test ./...
```
`make check` rewrites files via `gofmt -w`; for a non-mutating check use `go vet ./... && go test ./...`.

## Tenant isolation (non-negotiable)
- Every tenant-owned table has `photographer_id`; scope **every** query by it. RLS is a second net, not a replacement. No tenant context means no rows, so an unexpectedly empty result usually means the context helper was not called.
- Derive the ID from the authenticated session or validated magic link, never from request body/URL.
- The API connects as non-owner `photobooth_app` (`DATABASE_URL`, NOBYPASSRLS). Only migrations use the owner (`MIGRATE_DATABASE_URL`) — owners bypass RLS.
- Every tenant-data request runs in a transaction starting with `SELECT set_config('app.photographer_id', $1, true)` via the shared helper in `internal/platform/database` (not implemented yet). `true` keeps the setting transaction-local so it cannot leak across pooled connections. Never call it ad hoc.
- New tenant tables need `UNIQUE (photographer_id, id)`, composite FKs to parents, and an RLS policy copied from `migrations/00002_clients_and_shoots.sql`, plus a test proving a different photographer cannot access the data.
- Clients have no session: resolve links only via `resolve_gallery_link(token_hash)` (SECURITY DEFINER → photographer_id, shoot_id), then set tenant context. This is the only pre-tenant lookup allowed.

## Money (non-negotiable)
- Integer minor units (KES cents), never floats. Rates, quotas and retention are config in `plans`, not hard-coded constants.
- The ledger is append-only (a DB trigger rejects UPDATE/DELETE); corrections are new reversing entries. Entries sharing a `txn_id` must sum to zero. Snapshot commission on the payment; never recompute history.
- M-PESA callbacks arrive late, twice or out of order: save the raw callback first, dedupe on `(provider, provider_txn_id)`, make handlers idempotent and retryable. `payment_callbacks` is intentionally not tenant-scoped (tenant unknown on arrival).
- Only verified server-side callbacks/status queries mark payments paid or unlock originals — never the browser. Tests required for idempotency, commission and unlock-on-payment.
- delivered and expired are set only by system code; photographers set shot, editing and ready.

## SQL, secrets, retention
- All SQL lives in sqlc `queries.sql` files — no raw or string-concatenated SQL in Go. Multi-table writes use transactions. Schema changes only via a new goose migration with a working `Down`; never edit an applied migration.
- Never commit secrets or `.env`; keep `.env.example` current. Never log tokens, full phone numbers or payment credentials. Session, gallery-link, access-code and invitation tokens are stored hashed, long, random and expiring. Validate upload type/size; presigned upload URLs are short-lived and scoped to one object key. Originals are untouched; clients see generated previews only.
- Signup is invite-only. Invitation tokens are stored hashed and never logged.
- The admin sees platform metadata and aggregates only: never clients, shoots, photos or delivery links, and there is no impersonation. Admin routes use admin middleware only. Every admin action is audited.
- Never persist or log a delivery token or URL. Delivery links are sent inline, never from a background job.
- Never put an original's URL or key in any response served to clients before unlock. Storage keys never contain user text.
- Photo files, including previews and thumbnails, expire per plan: warn the client first, then delete. Keep client records, pending bills and approved testimonials (Kenya Data Protection Act).

## Testing
Tests live next to code; table-driven where useful. Must-have coverage: tenant isolation, ledger/commission, M-PESA callback idempotency, magic-link validation, unlock-on-payment. DB tests use real Postgres via `internal/platform/testdb`, not mocks. Fix the code or explain why a test is wrong — never weaken assertions.

## Frontend (web/ not scaffolded)
See `web/README.md` for scaffold commands. Read `docs/design-system.md` before any UI; tokens live in `web/theme.css` (Tailwind v4 `@theme inline`) — use tokens, never hex in components. Light theme for dashboard/marketing; add the `dark` class for client gallery/delivery. Volt is a fill with ink text on light, never text on light, and is used sparingly. Client pages are mobile-first and slow-data friendly. `docs/prototypes/client-delivery.html` is the reference for flow, states and copy — port it with Shadcn and real API calls. Only the palette is borrowed from landonorris.com; no images, logos, fonts or copy. Marketing copy lives in `web/lib/content/` and placeholder content is tracked in `docs/PLACEHOLDERS.md`. Browser code sees `NEXT_PUBLIC_*` only.

## Working style
Sole developer is learning Go: prefer simple readable code, explain non-obvious code and Go concepts, comment the why. Work in small vertical slices (migration → queries → handler → test → minimal UI); plan briefly before non-trivial work. Ask rather than guess on payments, permissions or deletion; payment/ledger code is reviewed line by line, so flag assumptions. No unrelated refactors. New config values go in `.env.example`; new conventions go in this file.

## Definition of done
`make check` passes; new tenant data is scoped and has an isolation test; schema changes are a new migration with `Down`; new config is in `.env.example`.

Deeper context: `docs/schema-notes.md` (schema intent), `docs/decisions/0001-architecture.md` (why).
