# web

Next.js frontend. Not scaffolded yet. From this folder's parent:

```
npx create-next-app@latest web      # TypeScript, App Router, Tailwind
cd web && npx shadcn@latest init
```

Then merge `theme.css` into `app/globals.css` (instructions at the top of that file).

Planned route groups (folders in parentheses don't change the URL):

```
app/(auth)/            login, signup
app/(dashboard)/       photographer area: shoots, clients, settings
app/(client)/g/[token]/  client gallery and delivery page (uses the `dark` class)
app/(public)/          photographer public profile, marketing pages
components/ui/         Shadcn components (generated)
components/            feature components (gallery-grid, pay-sheet, ...)
lib/                   API client, formatting helpers
```

Reference for the client page: `../docs/prototypes/client-delivery.html`.
Rules: see `../docs/design-system.md` and the frontend section of `../AGENTS.md`.
