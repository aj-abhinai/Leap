FROM node:22-alpine AS frontend
WORKDIR /build
COPY frontend/pnpm-lock.yaml frontend/package.json ./
RUN npm install -g pnpm && pnpm install --frozen-lockfile
COPY frontend/ .
RUN pnpm build

FROM golang:1.25-alpine AS backend
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /build/dist ./frontend/dist
RUN CGO_ENABLED=0 go build -ldflags="-X 'main.buildString=unknown' -X 'main.versionString=v0.1.0'" -o crm ./cmd/server/ \
    && go install github.com/knadh/stuffbin/stuffbin@latest \
    && "$(go env GOPATH)/bin/stuffbin" -a stuff -in crm -out crm.stuffed frontend/dist:/frontend/dist migrations:/migrations \
    && mv crm.stuffed crm

FROM alpine:latest
RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S -g 10001 crm \
    && adduser -S -D -H -u 10001 -G crm crm
WORKDIR /crm
COPY --from=backend /build/crm .
COPY config.toml .
USER crm
EXPOSE 9000
ENTRYPOINT ["./crm"]
