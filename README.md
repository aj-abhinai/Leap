# CRM

<br> Self-hosted pipeline CRM for internal teams. Contacts, kanban leads with configurable stages, role-based access control, and a full audit log — in a single binary.

## Documentation

For detailed documentation, read the [docs](https://aj-crm.pages.dev/).

## Features

- **Contacts**  
  The single source of truth for every client. Multiple phones and emails per contact with a primary, tags, custom statuses, notes, and CSV bulk import and export.

- **Pipeline kanban**  
  Configurable pipelines with custom stages. Drag leads between stages on a kanban board. Cards preview the next task and last touch, with overdue tasks flagged red.

- **Lead activities & reminders**  
  A timeline of tasks per lead — calls, follow-ups, notes. Quick replies capture outcomes and drive the flow: log only, auto-create the next task, or close the lead as lost. Reminders are snoozable and surface overdue work.

- **Program catalog**  
  Fixed-price programs managed in Settings. Lead values are price snapshots at creation time — catalog price changes never rewrite existing leads.

- **Dynamic RBAC**  
  Create custom roles with per-action permissions on contacts, leads, pipelines, users, and settings. Superadmin bootstraps on first run.

- **Audit log**  
  Every mutation is recorded with who did what and when, including a JSON diff of the changes. Read-only views and an activity feed for the whole team.

- **Secure sessions**  
  Short-lived JWT access tokens in memory; the refresh token lives in an HttpOnly cookie with CSRF protection on cookie-authenticated requests. Login and refresh are rate-limited.

- **Modern UI**  
  Vue 3, TypeScript, Tailwind CSS v4, and shadcn-vue. Light, dark, and system themes with no flash on first paint.

- **Single binary**  
  The Go backend embeds the Vue frontend and database migrations via stuffbin. Deploy with one `docker compose` command — no separate asset directories.

## Tech stack

| Layer | Choices |
|---|---|
| Backend | Go 1.25+ (chi router, pgx, raw SQL, JWT, golang-migrate) |
| Frontend | Vue 3, TypeScript, Tailwind CSS v4, shadcn-vue, Pinia, TanStack Query |
| Database | PostgreSQL 17 |

## Installation

### Docker (development)

```shell
# One command — dev config, loopback binds, admin/admin bootstrap
just docker-up
```

Go to `http://localhost:9000` and log in with `admin@admin.com` / `admin`.

> Development-only bootstrap credentials, reachable only through the dev startup paths.
> Production refuses them.

### Docker (production)

```shell
# 1. Copy and edit the production env file (compose reads it from docker/)
cp docker/.env.example docker/.env
# 2. Edit docker/.env — set JWT_SECRET, SUPERADMIN_EMAIL, SUPERADMIN_PASSWORD

# 3. Start (the app exits immediately if a secret is missing or a placeholder remains)
docker compose -f docker/docker-compose.yml up -d
```

Go to `http://localhost:9000` and log in with your `SUPERADMIN_EMAIL` / `SUPERADMIN_PASSWORD`.

Bare `docker compose` always runs **production**: no dev config is mounted, and startup
validation rejects the committed development credentials, the burned JWT secret, and any
placeholder. Development uses the compose override (`just docker-up`) instead.

### Binary

```shell
# 1. Generate a config file (overwrites only a pristine template, never edits)
./crm --new-config config.toml
# 2. Edit config.toml — replace every placeholder secret (the server
#    refuses to start with placeholder/empty secrets)
# 3. Start PostgreSQL externally and run
./crm
```

Migrations run automatically and idempotently at startup. The binary contains the frontend
and migrations packed via stuffbin, so it runs anywhere without extra asset directories.

### Build from source

```shell
# Requires Go 1.25+, Node 22+, pnpm, and a running Postgres
just build
```

The binary is at `bin/crm`. Unstuffed development builds automatically fall back to the
local `frontend/dist` and `migrations` directories.

## Configuration

Two config files are committed, each with a single purpose:

| File | Purpose |
|---|---|
| `config.toml` | **Production template** (`environment = "production"`). Ships placeholders that startup validation refuses — not bootable until real secrets are supplied. |
| `config.dev.toml` | **Development only** (`environment = "development"`). Carries the fixed bootstrap credentials and the burned dev JWT secret. Reachable only through dev startup paths; validation rejects it outside development. |

Secrets can be supplied via environment variables, which override the config file:

| Environment variable | Overrides |
|---|---|
| `APP_PORT` / `APP_ENV` | `[app] port` / `environment` |
| `DB_HOST` / `DB_PORT` / `DB_USER` / `DB_PASSWORD` / `DB_NAME` / `DB_SSLMODE` | `[db] *` |
| `JWT_SECRET` / `JWT_ISSUER` | `[auth] jwt_secret` / `jwt_issuer` |
| `COOKIE_SECURE` | `[auth] secure_cookies` — set `true` when serving over HTTPS |
| `SUPERADMIN_EMAIL` / `SUPERADMIN_PASSWORD` | `[superadmin] *` |

Required secrets: `JWT_SECRET` (at least 32 characters) and a superadmin email with a
password of at least 12 characters. The application refuses to start with placeholder,
empty, or known development secrets in production. Point `DB_*` at any existing PostgreSQL
instance to run the app without the bundled `db` service.

## Development

```shell
# Start Postgres in Docker (one-time, stays running)
just dev-db

# Backend (in one terminal)
just dev-backend

# Frontend (in a separate terminal — proxies /api to localhost:9000)
just dev-frontend

# Run tests
just test            # with coverage, no race detector (CGO-free)
just test-race       # with the race detector (requires a C compiler)
just test-frontend   # Vue component tests via Vitest

# Lint and format
just fmt vet lint
just check           # fmt + vet + lint + test

# Build the binary
just build

# Docker helpers
just docker-up       # dev stack: start app + Postgres (loopback binds)
just docker-up-prod  # production stack: fails fast until docker/.env secrets are set
just docker-rebuild  # rebuild image and start
just docker-down     # stop the stack
just docker-reset    # stop and remove volumes (destructive)
```

The Vite dev server at `localhost:5173` proxies `/api/*` to the Go backend at `localhost:9000`.

Health endpoints: `GET /healthz` (process alive) and `GET /readyz` (database ping).

## License
CRM is distributed under the terms of the AGPLv3 License.  
&copy; [Abhinai](https://abhinai.pages.dev/)  
## Support  
<a href="https://buymeacoffee.com/aj_abhinai" target="_blank"><img src="https://cdn.buymeacoffee.com/buttons/v2/default-yellow.png" alt="Buy Me A Coffee" style="height: 60px !important;width: 217px !important;" ></a>
