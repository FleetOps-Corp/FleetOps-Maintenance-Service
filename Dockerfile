# =============================================================================
# Multi-stage Dockerfile for FleetOps Maintenance Microservice
# SAD Reference: ADR-8 — Docker containers on AWS ECS
# Rule R5: Multi-stage build, pinned versions, non-root user
# =============================================================================

# -----------------------------------------------------------------------------
# Stage 1: Builder
# -----------------------------------------------------------------------------
FROM golang:1.22.4-alpine3.20 AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /build

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o /build/maintenance-service ./cmd/server

# -----------------------------------------------------------------------------
# Stage 2: Runtime
# -----------------------------------------------------------------------------
FROM alpine:3.20.1

# Install CA certificates for HTTPS calls to external services
RUN apk add --no-cache ca-certificates tzdata \
    # Create non-root user (Rule R5)
    addgroup -S appgroup && adduser -S appuser -G appgroup

WORKDIR /app

# Copy ONLY the compiled binary and migrations (not source code)
COPY --from=builder /build/maintenance-service .
COPY --from=builder /build/migrations ./migrations

# Switch to non-root user
USER appuser

# Expose the server port
EXPOSE 8080

# Health check for Docker and AWS ECS
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -qO- http://localhost:8080/health || exit 1

ENTRYPOINT ["./maintenance-service"]
