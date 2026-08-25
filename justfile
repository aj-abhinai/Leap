# CRM build commands — https://github.com/casey/just

# Build variables
exe := if os_family() == "windows" { ".exe" } else { "" }
bin := "bin/crm" + exe

# Config file (development default; production uses config.toml via --new-config)
config := env_var_or_default('CONFIG', 'config.dev.toml')

# Docker compose base + dev override (loopback binds, committed dev config)
compose := "docker compose -f docker/docker-compose.yml -f docker/docker-compose.dev.yml"
# Bare compose = production (fails fast until .env secrets are supplied)
compose_prod := "docker compose -f docker/docker-compose.yml"

# show available commands
default:
    @just --list

# === Dev ===

# Start Postgres in Docker, wait until ready, and free port :9000 (used by `backend` and `dev`).
[private]
_pg-up:
    #!/usr/bin/env sh
    set -e
    {{ compose }} up -d db
    echo "Waiting for Postgres..."
    for i in $(seq 1 30); do
      if docker exec crm_db pg_isready -U crm -d crm >/dev/null 2>&1; then
        echo "Postgres ready"
        break
      fi
      sleep 1
    done
    if ! docker exec crm_db pg_isready -U crm -d crm >/dev/null 2>&1; then
      echo "ERROR: Postgres did not become ready in 30s"
      {{ compose }} rm -sf db
      exit 1
    fi
    # Free port :9000 if the docker app container is running (avoids bind collision with `just docker-up`).
    {{ compose }} stop app >/dev/null 2>&1 || true

# Start Postgres in Docker and run the Go server. Ctrl+C stops Postgres.
backend: _pg-up
    #!/usr/bin/env sh
    set -e
    CLEANED_UP=""
    cleanup() { [ -n "$CLEANED_UP" ] && return; CLEANED_UP=1; echo " Stopping Postgres..."; {{ compose }} rm -sf db; }
    trap cleanup EXIT INT TERM
    go run ./cmd/server/ -config {{ config }}

# Start Postgres, then the backend and frontend dev servers in one command. Ctrl+C stops everything.
dev: _pg-up
    #!/usr/bin/env sh
    set -e
    BACKEND_PID=""
    FRONTEND_PID=""
    CLEANED_UP=""
    cleanup() {
      [ -n "$CLEANED_UP" ] && return
      CLEANED_UP=1
      if [ -n "$FRONTEND_PID" ]; then kill "$FRONTEND_PID" 2>/dev/null || true; fi
      if [ -n "$BACKEND_PID" ]; then kill "$BACKEND_PID" 2>/dev/null || true; fi
      echo " Stopping Postgres..."
      {{ compose }} rm -sf db
    }
    trap cleanup EXIT INT TERM
    echo "Starting backend on :9000..."
    go run ./cmd/server/ -config {{ config }} &
    BACKEND_PID=$!
    echo "Starting frontend on :5173..."
    (cd frontend && ([ -d "node_modules" ] || pnpm install --frozen-lockfile) && pnpm dev) &
    FRONTEND_PID=$!
    wait

# Vite dev server with HMR on :5173 (proxies /api to :9000)
frontend:
    cd frontend && \
    [ -d "node_modules" ] || pnpm install --frozen-lockfile && \
    pnpm dev

# Start only Postgres in Docker, detached (with the dev override for loopback publish)
dev-db:
    {{ compose }} up -d db

# === Build ===

# Build frontend then backend (single binary with embedded SPA)
build: build-ui build-backend

# Build Go binary with stuffbin-packed frontend
build-backend:
    @echo "Building backend..."
    CGO_ENABLED=0 go build -ldflags="-X 'main.buildString=dev' -X 'main.versionString=v0.1.0'" -o {{ bin }} ./cmd/server/
    @echo "Packing frontend into binary..."
    MSYS_NO_PATHCONV=1 stuffbin -a stuff -in {{ bin }} -out {{ bin }}.stuffed frontend/dist:/frontend/dist migrations:/migrations
    mv {{ bin }}.stuffed {{ bin }}

# Build frontend production bundle
build-ui:
    @echo "Building frontend..."
    cd frontend && \
    [ -d "node_modules" ] || pnpm install --frozen-lockfile && \
    pnpm build

# === Docker ===

# Development stack: loopback binds + committed config.dev.toml (admin/admin).
docker-up:
    {{ compose }} up -d

# Production stack: bare compose, fails fast until docker/.env secrets are set.
docker-up-prod:
    {{ compose_prod }} up -d

# Build (or rebuild) the app image without starting containers
docker-build:
    {{ compose }} build

# Rebuild the image and start the dev stack
docker-rebuild:
    {{ compose }} up --build -d

docker-down:
    {{ compose }} down

docker-reset:
    {{ compose }} down -v

# === Tests / lint ===

test:
    @echo "Running tests with coverage..."
    mkdir -p coverage
    go test -v -coverprofile=coverage/coverage.out ./... && \
    go tool cover -html=coverage/coverage.out -o coverage/coverage.html
    @echo "Coverage report generated at coverage/coverage.html"
    @go tool cover -func=coverage/coverage.out | grep total | awk '{print "Total coverage: " $$3}'

# Run tests with the race detector (requires CGO + a C compiler, e.g. gcc/mingw on Windows)
test-race:
    @echo "Running tests with race detection and coverage..."
    mkdir -p coverage
    go test -v -race -coverprofile=coverage/coverage.out ./... && \
    go tool cover -html=coverage/coverage.out -o coverage/coverage.html
    @echo "Coverage report generated at coverage/coverage.html"
    @go tool cover -func=coverage/coverage.out | grep total | awk '{print "Total coverage: " $$3}'

test-short:
    @echo "Running tests (short mode)..."
    go test -v ./...

test-frontend:
    cd frontend && pnpm test:run

lint:
    golangci-lint run

fmt:
    go fmt ./...

vet:
    go vet ./...

tidy:
    go mod tidy

check: fmt vet lint test

# === Cleanup ===

clean:
    @echo "Cleaning build artifacts..."
    rm -rf bin coverage frontend/dist/ frontend/node_modules/.vite

clean-all: clean
    @echo "Removing node_modules..."
    rm -rf frontend/node_modules
