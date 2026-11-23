# 0001: Architecture and early decisions

Status: accepted (Nov 2026). Revisit after market research.

## Decisions

- **Modular monolith.** One Go service organised into domain packages under `internal/`, plus a Next.js frontend. Sole developer; microservices would cost more than they return.
- **Go + sqlc + pgx + goose.** Plain SQL stays visible and the developer is learning Go. No ORM (Prisma was dropped).
- **Postgres with row-level security** as a second safety net behind explicit `photographer_id` scoping. Composite foreign keys prevent cross-tenant links.
- **River** (Postgres-backed) for background jobs. No Redis until caching or rate limiting is needed.
- **Cloudflare R2** for photos, with presigned multipart uploads straight from the browser. Originals are untouched; clients browse watermarked previews.
- **Multi-tenant product** sold to other photographers.
- **Pricing:** hybrid. Commission on paid deliveries plus optional paid plans. Values are placeholders held in the `plans` table; to be set after a sample-size survey of photographers.
- **Payments:** M-PESA STK push. Commission requires being in the payment path, so either collect and pay out, or use a licensed split-payment partner (preferred for v1). Needs legal advice before launch.
- **Unlock flow:** watermarked previews, client approves and pays, originals unlock server-side after a verified callback.
- **Visual identity:** palette borrowed from landonorris.com (colours only). Client pages use a dark theme.

## Open questions

- Payment route: self-collect vs licensed partner.
- Final pricing and plan limits.
- Typeface.
- "Request a change" path for unhappy clients.
- Data Protection Act registration and retention policy wording.
