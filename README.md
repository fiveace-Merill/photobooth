# Photobooth

A platform for photographers to deliver photos to clients: watermarked previews, approve and pay with M-PESA, then originals unlock. Sold to photographers on a commission-based hybrid plan.

## Quick start

```
cp .env.example .env        # then change the passwords
make db-up
make migrate-up
make run                    # http://localhost:8080/healthz
```

Requires: Go 1.22+, Docker, [goose](https://github.com/pressly/goose), [sqlc](https://sqlc.dev), Node 20+.
Run `make help` for all commands.

## Where things are

See `AGENTS.md` for the full layout and rules, `docs/` for design notes, and `docs/decisions/` for why things are the way they are.
# photobooth
