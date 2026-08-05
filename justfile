# CRM build commands — https://github.com/casey/just

# Build variables
exe := if os_family() == "windows" { ".exe" } else { "" }
bin := "bin/crm" + exe

# Config file
config := env_var_or_default('CONFIG', 'config.toml')

# show available commands
default:
    @just --list

# === Dev ===

# Start Postgres in Docker and run the Go server. Ctrl+C stops Postgres.
dev-backend:
    #!/usr/bin/env sh
    set -e
    CLEANED_UP=""
    cleanup() { [ -n "$CLEANED_UP" ] && return; CLEANED_UP=1; echo " Stopping Postgres..."; docker compose -f docker/docker-compose.yml rm -sf db; }
    trap cleanup EXIT INT TERM
    docker compose -f docker/docker-compose.yml up -d db
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
      exit 1
    fi
    # Free port :9000 if the docker app container is running (avoids bind collision with `just docker-up`).
    docker compose -f docker/docker-compose.yml stop app >/dev/null 2>&1 || true
    go run ./cmd/server/ -config {{config}}

# Same as dev-backend
dev: dev-backend

# Vite dev server with HMR on :5173 (proxies /api to :9000)
dev-frontend:
    cd frontend && \
    [ -d "node_modules" ] || pnpm install --frozen-lockfile && \
    pnpm dev

# Start only Postgres in Docker, detached
dev-db:
    docker compose -f docker/docker-compose.yml up -d db

# === Build ===

# Build frontend then backend (single binary with embedded SPA)
build: build-ui build-backend

# Build Go binary with stuffbin-packed frontend
build-backend:
    @echo "Building backend..."
    CGO_ENABLED=0 go build -ldflags="-X 'main.buildString=dev' -X 'main.versionString=v0.1.0'" -o {{bin}} ./cmd/server/
    @echo "Packing frontend into binary..."
    MSYS_NO_PATHCONV=1 stuffbin -a stuff -in {{bin}} -out {{bin}}.stuffed frontend/dist:/frontend/dist migrations:/migrations
    mv {{bin}}.stuffed {{bin}}

# Build frontend production bundle
build-ui:
    @echo "Building frontend..."
    cd frontend && \
    [ -d "node_modules" ] || pnpm install --frozen-lockfile && \
    pnpm build

# === Docker ===

docker-up:
    docker compose -f docker/docker-compose.yml up -d

# Build (or rebuild) the app image without starting containers
docker-build:
    docker compose -f docker/docker-compose.yml build

# Rebuild the image and start the stack
docker-rebuild:
    docker compose -f docker/docker-compose.yml up --build -d

docker-down:
    docker compose -f docker/docker-compose.yml down

docker-reset:
    docker compose -f docker/docker-compose.yml down -v

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
