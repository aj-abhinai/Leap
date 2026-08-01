# Prayaan CRM

> A simple, single-tenant pipeline CRM for internal teams — contacts, leads with configurable stages, and role-based access control, all in a single binary.

---

## Features

- **Contact management** — Track name, email, phone, location, and age for every contact.
- **Pipeline-based leads** — Configurable pipelines with custom stages. Drag leads between stages on a kanban board.
- **Program catalog** — Fixed-price programs with a settings tab; lead values are price snapshots at creation time and never rewrite when catalog prices change.
- **Dynamic RBAC** — Create custom roles with granular permissions. Superadmin bootstraps on first run.
- **Activity logs** — Every mutation is tracked with who did what and when.
- **Secure sessions** — Short-lived access tokens in memory; the refresh token lives in an HttpOnly cookie, with CSRF protection on cookie-authenticated requests.
- **Single binary** — Go backend embeds the Vue frontend via stuffbin. Deploy with one `docker compose` command.

---

## Tech stack

| Layer | Choices |
|---|---|
| Backend | Go 1.25+ (chi router, pgx, raw SQL, JWT, golang-migrate) |
| Frontend | Vue 3, TypeScript, Tailwind CSS v4, shadcn-vue, Pinia, TanStack Query |
| Database | PostgreSQL 17 |

---

## Quick start

### Docker

```shell
# Copy sample env and edit secrets
cp .env.example .env
# Edit .env — set JWT_SECRET, superadmin credentials

# Start the app + Postgres
docker compose -f docker/docker-compose.yml up -d
```

Go to `http://localhost:9000` and login with the superadmin credentials from your `.env`.

### Build from source

```shell
# Requires Go 1.25+, Node 22+, pnpm, and a running Postgres
just build
```

The binary is at `bin/crm` — it contains the frontend and migrations packed via
[stuffbin](https://github.com/knadh/stuffbin), so it runs anywhere without extra asset
directories. Unstuffed development builds automatically fall back to the local
`frontend/dist` and `migrations` directories.

---

## Deployment

### Option 1 — downloaded binary + existing PostgreSQL

```shell
# 1. Generate a config file (refuses to overwrite an existing file)
./crm --new-config config.toml
# 2. Edit config.toml — replace every placeholder secret (the server
#    refuses to start with placeholder/empty secrets)
# 3. Start PostgreSQL externally and run
./crm
```

Migrations run automatically and idempotently at startup. Secrets can alternatively be
supplied via environment variables, which override `config.toml`:

| Environment variable | Overrides |
|---|---|
| `APP_PORT` | `[app] port` |
| `DB_HOST` / `DB_PORT` / `DB_USER` / `DB_PASSWORD` / `DB_NAME` / `DB_SSLMODE` | `[db] *` |
| `JWT_SECRET` / `JWT_ISSUER` | `[auth] jwt_secret` / `jwt_issuer` |
| `COOKIE_SECURE` | `[auth] secure_cookies` — set `true` when serving over HTTPS |
| `SUPERADMIN_EMAIL` / `SUPERADMIN_PASSWORD` | `[superadmin] *` |

### Option 2 — default Docker Compose (bundled PostgreSQL)

```shell
cp .env.example .env
# Edit .env — set JWT_SECRET and superadmin credentials (placeholders fail fast)
docker compose -f docker/docker-compose.yml up -d
```

Go to `http://localhost:9000` and login with the superadmin credentials from your `.env`.

### Using an existing PostgreSQL VM with Docker

Stop/remove the bundled `db` service, then pass connection variables instead:

```shell
DB_HOST=10.0.0.5 DB_PORT=5432 DB_USER=crm DB_PASSWORD=secret \
DB_NAME=crm docker compose -f docker/docker-compose.yml up -d app
```

The same stuffed binary is used for Docker and standalone downloads — the container needs
no separate `frontend/dist` or `migrations` directories. PostgreSQL itself is never
bundled into the application.

### Health endpoints

- `GET /healthz` — 200 when the process is alive
- `GET /readyz` — 200 when the database ping succeeds, 503 otherwise

---

## Development

```shell
# Start Postgres in Docker (one-time, stays running)
just dev-db

# Backend (in one terminal)
just dev-backend

# Frontend (in a separate terminal — proxies /api to localhost:9000)
just dev-frontend

# Run tests
just test

# Build the binary
just build
```

The Vite dev server at `localhost:5173` proxies `/api/*` to the Go backend at `localhost:9000`.

> **Note:** `just dev-db` starts only the Postgres container. Stop it with `docker compose -f docker/docker-compose.yml down`.

---

## Configuration

| Section | Key | Default | Description |
|---|---|---|---|
| `[app]` | `port` | `9000` | HTTP listen port |
| `[app]` | `name` | `CRM` | Application name |
| `[db]` | `host` | `localhost` | PostgreSQL host |
| `[db]` | `port` | `5432` | PostgreSQL port |
| `[db]` | `user` | `crm` | Database user |
| `[db]` | `password` | `crm` | Database password |
| `[db]` | `name` | `crm` | Database name |
| `[db]` | `sslmode` | `disable` | SSL mode |
| `[auth]` | `jwt_secret` | dev placeholder | **Required.** At least 32 characters, never a placeholder. |
| `[auth]` | `access_token_ttl` | `15m` | Access token lifetime |
| `[auth]` | `refresh_token_ttl` | `168h` | Refresh token lifetime |
| `[auth]` | `bcrypt_cost` | `12` | Password hash cost |
| `[auth]` | `secure_cookies` | `false` | Add the `Secure` flag to auth cookies; set `true` behind HTTPS |
| `[superadmin]` | `email` | dev placeholder | **Required.** Seeded superadmin email |
| `[superadmin]` | `password` | dev placeholder | **Required.** At least 12 characters, never a placeholder |

All secrets can be overridden via environment variables — see `.env.example` and the
Deployment section. The application refuses to start with placeholder or empty secrets in
every environment.

---

## Architecture decisions

All architecture decisions are documented in [`docs/adr/`](docs/adr/).

---

## License

MIT &copy; Prayaan OS
