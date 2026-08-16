---
title: Quickstart
description: Install and run CRM in minutes
---

This guide is a procedure. It shows how to install CRM and start it.

You can run CRM in three ways:

- with Docker Compose (development),
- with Docker Compose (production),
- as a single binary.

## Prerequisites

- Docker and Docker Compose (for the Docker options).
- Go 1.25+ and Node 22+ (for a build from source).
- pnpm (for a build from source).

## Run with Docker Compose (development)

1. Start the development stack:

   ```shell
   just docker-up
   ```

2. Open `http://localhost:9000` in a browser.
3. Log in with `admin@admin.com` and the password `admin`.

> The credentials above work only in the development stack. Production refuses them.

## Run with Docker Compose (production)

1. Copy the example environment file:

   ```shell
   cp docker/.env.example docker/.env
   ```

2. Edit `docker/.env`. Set `JWT_SECRET`, `SUPERADMIN_EMAIL`, and `SUPERADMIN_PASSWORD`.
3. Start the stack:

   ```shell
   docker compose -f docker/docker-compose.yml up -d
   ```

4. Open `http://localhost:9000` in a browser.
5. Log in with your `SUPERADMIN_EMAIL` and `SUPERADMIN_PASSWORD`.

> If a secret is missing, or a placeholder remains, the application exits immediately. It starts only when every secret is real.

## Run as a single binary

1. Generate a config file:

   ```shell
   ./crm --new-config config.toml
   ```

2. Edit `config.toml`. Replace every placeholder secret.
3. Start PostgreSQL. You can use any existing instance.
4. Run the binary:

   ```shell
   ./crm
   ```

The migrations run automatically at startup. They are idempotent, so a restart is safe.

> The server refuses to start with placeholder or empty secrets.

## Build from source

1. Start PostgreSQL in Docker:

   ```shell
   just dev-db
   ```

2. Build the binary:

   ```shell
   just build
   ```

The binary is at `bin/crm`. It contains the frontend and the migrations, so it runs anywhere without extra asset directories.

## What is next?

- Read the [API Reference](/api-reference/overview/) to explore the REST API.
- Run the tests with `just check`. The command runs format, vet, lint, and tests.
