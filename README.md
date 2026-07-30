# Prayaan CRM

> A simple, single-tenant pipeline CRM for internal teams — contacts, leads with configurable stages, and role-based access control, all in a single binary.

---

## Features

- **Contact management** — Track name, email, phone, location, and age for every contact.
- **Pipeline-based leads** — Configurable pipelines with custom stages. Drag leads between stages on a kanban board.
- **Dynamic RBAC** — Create custom roles with granular permissions. Superadmin bootstraps on first run.
- **Activity logs** — Every mutation is tracked with who did what and when.
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

The binary is at `bin/crm`.

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
| `[auth]` | `jwt_secret` | — | **Required.** Change in production. |
| `[auth]` | `access_token_ttl` | `15m` | Access token lifetime |
| `[auth]` | `refresh_token_ttl` | `168h` | Refresh token lifetime |
| `[auth]` | `bcrypt_cost` | `12` | Password hash cost |

Secrets (`DB_PASSWORD`, `JWT_SECRET`, superadmin credentials) can also be set via environment variables — see `.env.example`.

---

## Architecture decisions

All architecture decisions are documented in [`docs/adr/`](docs/adr/).

---

## License

MIT &copy; Prayaan OS
