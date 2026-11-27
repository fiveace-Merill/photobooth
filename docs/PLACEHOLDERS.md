# Placeholders

This file tracks placeholder content that must be replaced before a public launch or real billing rollout. Keep it updated when placeholder content is added, moved, or removed.

| Placeholder | File(s) | Replace with | Notes |
|---|---|---|---|
| Fake contact email `hello@photobooth.example` | `web/lib/site.ts`, privacy/marketing UI | Real support/contact email | Used in early-access contact line and privacy draft. |
| Fake WhatsApp number `+254700000000` | `web/lib/site.ts`, marketing UI | Real WhatsApp/business number | Displayed on marketing page. |
| Placeholder site URL `https://photobooth.example` | `web/lib/site.ts` | Real production origin | Used for canonical/OG metadata. |
| Sample testimonials (`isPlaceholder: true`) | `web/lib/content/testimonials.ts` | Real photographer quotes with permission | Clearly labeled SAMPLE on the page; do not launch publicly as real testimonials. |
| Placeholder plan/pricing values | `web/lib/content/plans.ts`, `migrations/00001_tenants_and_auth.sql` | Survey-backed pricing/limits | Amounts are minor units/bps; DB seeds and UI content must stay aligned. |
| Draft privacy policy text | `web/app/(marketing)/privacy/page.tsx` | Legally reviewed privacy policy | Page is marked DRAFT pending legal review. |
| Demo delivery links/content | `web/lib/site.ts` (`DEMO_PATH`), `web/lib/api/mock.ts` | Real gallery links and API data | Demo route `/g/demo` is intentional for marketing; keep `noindex` on client route group. |
| Temporary stock/demo images | `web/public/demo/`, `web/public/images/`, `docs/image-credits.md` | Licensed/owned production images with confirmed credits | Stock Unsplash images are temporary; previews generated in `web/public/demo/previews/`; originals used only by unlocked mock data. |

| Placeholder sender address `no-reply@photobooth.local` | `.env.example`, local email config | Real verified sending address/domain | Used as `MAIL_FROM_ADDRESS`; must be a domain controlled by Photobooth. Real inbox delivery/spam placement need real provider/domain. |
| Placeholder sender name `Photobooth` | `.env.example`, email config | Final product sender name | Used in `"{Studio} via {MAIL_FROM_NAME}"` From header. |
| Local Mailpit SMTP/UI | `docker-compose.yml`, `.env.example` | Real email provider (SMTP/API) and verified domain | Mailpit is for local testing only; open `http://localhost:8025`. |
