# Stage 1: Build the binaries using Golang
FROM golang:alpine AS builder

WORKDIR /app

# Install build essentials
RUN apk add --no-cache ca-certificates git tzdata

# Leverage Docker layer caching for dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy application source code
COPY . .

# Build statically linked binaries for api, migrate, and seed
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /app/bin/cadence-api ./cmd/api
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /app/bin/cadence-migrate ./cmd/migrate
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /app/bin/cadence-seed ./cmd/seed

# Stage 2: Minimal hardened runtime image
FROM alpine:3.20

WORKDIR /app

# Install runtime dependencies (TLS certificates, timezone data, pg_dump for backups)
RUN apk add --no-cache ca-certificates tzdata postgresql16-client

# Create an unprivileged non-root user and group
RUN addgroup -S cadence && adduser -S -G cadence -u 10001 cadence

# Copy binaries and migration SQL files from builder
COPY --from=builder /app/bin/cadence-api /app/cadence-api
COPY --from=builder /app/bin/cadence-migrate /app/cadence-migrate
COPY --from=builder /app/bin/cadence-seed /app/cadence-seed
COPY --from=builder /app/migrations /app/migrations

# Set file permissions for the unprivileged user
RUN chown -R cadence:cadence /app

USER cadence

EXPOSE 8083

ENTRYPOINT ["/app/cadence-api"]
