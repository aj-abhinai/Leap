FROM node:22-alpine AS frontend
WORKDIR /build
COPY frontend/pnpm-lock.yaml frontend/package.json ./
RUN npm install -g pnpm && pnpm install --frozen-lockfile
COPY frontend/ .
RUN pnpm build

FROM golang:1.24-alpine AS backend
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /build/dist ./frontend/dist
RUN CGO_ENABLED=0 go build -ldflags="-X 'main.buildString=unknown' -X 'main.versionString=v0.1.0'" -o crm ./cmd/server/

FROM alpine:latest
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /crm
COPY --from=backend /build/crm .
COPY --from=backend /build/migrations ./migrations
COPY config.toml .
EXPOSE 9000
ENTRYPOINT ["./crm"]
