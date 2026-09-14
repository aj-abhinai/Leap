---
title: Development
description: Set up a local environment for Leap
---

This guide is a procedure. It shows how to set up a local environment for Leap.

## Prerequisites

- Docker and Docker Compose.
- `just`.
- Go 1.25+.
- Node 22+.
- pnpm.

## First-time setup

1. Clone the repository.
2. Start the development stack:

   ```shell
   just dev
   ```

   The command starts PostgreSQL and pgweb in Docker, the Go backend on `:9000`, and the Vite dev server on `:5173`. The first run installs the frontend dependencies.

3. Open `http://localhost:5173` in a browser.
4. Log in with `admin@admin.com` and the password `admin`.

Press Ctrl+C to stop the processes and remove the containers.

> The development stack uses the committed `config.dev.toml`. It carries fixed development credentials and a development JWT secret. Production validation rejects these values.

## Run modes

`just dev` starts every service in one terminal. You can also start the parts separately:

| Command | Services |
|---|---|
| `just dev` | PostgreSQL, pgweb, backend, frontend |
| `just backend` | PostgreSQL, pgweb, backend |
| `just frontend` | Vite dev server only (proxies `/api` to `:9000`) |
| `just dev-db` | PostgreSQL and pgweb, detached |

The development ports:

| Service | URL |
|---|---|
| Leap application | `http://localhost:9000` |
| Vite dev server | `http://localhost:5173` |
| pgweb (database browser) | `http://localhost:8081` |
| PostgreSQL | `localhost:5432` |

## Tests and lint

- `just test` runs the Go tests with a coverage report.
- `just test-race` adds the race detector. The command needs a C compiler.
- `just test-frontend` runs the Vue component tests with Vitest.
- `just check` runs format, vet, lint, and tests.
- `just fmt vet lint` runs the individual checks.

## Build

`just build` builds the frontend, then the Go binary, and packs the frontend and the migrations into the binary with stuffbin. The result is `bin/crm`.

- `just build-ui` builds the frontend only.
- `just build-backend` builds the binary and packs the assets.

## Database

The migrations run at the server startup, and they are idempotent. A restart is safe.

The development database runs in Docker with these values:

- host: `localhost`
- port: `5432`
- user: `crm`
- password: `crm`
- database: `crm`

Use pgweb at `http://localhost:8081` to browse the tables.

## Docker helpers

- `just docker-up` starts the development stack in Docker: the application, PostgreSQL, and pgweb.
- `just docker-down` stops the stack.
- `just docker-reset` stops the stack and removes the database volume.
- `just docker-rebuild` rebuilds the image and starts the stack.

> CAUTION: `just docker-reset` erases all data in the development database.
