# Design system

Read this before building or changing any UI. Tokens live in `web/theme.css`.

## Direction

The visual direction is borrowed from the colour palette of landonorris.com: a hard, high-contrast pairing of a near-black with a vivid lime, softened by an off-white, a deep olive-green and a warm grey. Only the **palette** is borrowed. Do not use that site's photos, logos, signature, helmets, fonts or copy.

## Palette

The hex values come from third-party design-showcase listings of the site (Awwwards lists `#D2FF00` and `#111112`; DesignRush lists `#CBF902`, `#2D3126`, `#F4F3F1`, `#7F8177`). They were not read from the site's own CSS, so treat them as close approximations. If you want exact values, inspect the live site in browser DevTools and update `web/theme.css`.

| Name | Hex | Role |
|---|---|---|
| Ink | `#111112` | Text on light, background on dark, primary button on light |
| Volt | `#D2FF00` | Brand highlight, primary button on dark, focus ring on dark |
| Paper | `#F4F3F1` | Background on light, text on dark |
| Deep green | `#2D3126` | Borders and secondary surfaces on dark, focus ring on light |
| Stone | `#7F8177` | Decorative grey, large text, icons |

Derived (not from the source site): card `#FAFAF8` / `#191A17`, muted text `#5E6056` (light) and `#9A9C91` (dark), hover surface `#E8E7E3`, border `#D9D8D3`, destructive `#B3261E` / `#FF6B5E`.

## Contrast (WCAG ratios, calculated)

| Pair | Ratio | Verdict |
|---|---|---|
| Ink on paper | 17.0 | Pass |
| Ink on volt | 16.2 | Pass |
| Volt on ink | 16.2 | Pass |
| Paper on deep green | 12.0 | Pass |
| Volt on deep green | 11.4 | Pass |
| Muted text `#5E6056` on paper | 5.8 | Pass |
| Muted text `#9A9C91` on ink | 6.8 | Pass |
| Stone `#7F8177` on ink | 4.8 | Pass (borderline) |
| **Stone `#7F8177` on paper** | **3.6** | **Fails for body text.** Large text and icons only |
| **Volt on paper** | **1.05** | **Unreadable.** Never use as text or a thin line on light |

## Rules

1. **Two modes, by route.** Dashboard and marketing pages use the light theme. Client gallery and delivery pages add the `dark` class, because photographs read best on a dark ground.
2. **Volt is a fill, not a text colour on light.** On light backgrounds it appears only as a filled shape (badge, highlight, selected state) with ink text on top. On dark backgrounds it can be text, buttons and rings.
3. **Primary button:** ink with paper text on light; volt with ink text on dark.
4. **Use `bg-volt` / `text-volt-foreground` for brand highlights** (for example a "Paid" or "Unlocked" chip). Keep shadcn's `accent` for quiet hover surfaces.
5. **Use tokens, never hex values, in components.**
6. **Spend volt sparingly.** One or two lime moments per screen keeps it special.
7. Visible keyboard focus on everything interactive, and respect `prefers-reduced-motion`.

## Per-photographer branding (later)

Photographers will want their own look on their public profile and client galleries. The token approach supports this: override `--primary`, `--volt` and the logo per tenant via CSS variables loaded from the API. The Photobooth palette above is the default and the platform's own identity (dashboard, marketing, login).

## Open decisions

- **Typography:** not chosen yet. Pick one or two typefaces deliberately (via `next/font`), not framework defaults.
- **Imagery and shapes:** the source site uses organic blob shapes. Decide whether Photobooth borrows the idea or stays plain.
- **Motion:** none planned beyond responses to user actions.
- **Logo and product name:** "Photobooth" is a working title.
