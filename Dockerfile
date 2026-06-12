# Build stage
FROM golang:1.24-alpine AS builder

# Install build dependencies for CGO (required for go-sqlite3)
RUN apk add --no-cache gcc musl-dev

WORKDIR /app

# Copy go module files first for better layer caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build both binaries
RUN CGO_ENABLED=1 GOOS=linux go build -ldflags="-w -s" -o /app/bin/stream ./cmd/stream
RUN CGO_ENABLED=1 GOOS=linux go build -ldflags="-w -s" -o /app/bin/webhook ./cmd/webhook

# Final stage
FROM alpine:latest

# Install runtime dependencies
RUN apk add --no-cache ca-certificates sqlite tzdata

# Create non-root user
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

WORKDIR /app

# Copy binaries from builder
COPY --from=builder /app/bin/stream /app/stream
COPY --from=builder /app/bin/webhook /app/webhook

# Create data directory for SQLite database
RUN mkdir -p /data && chown appuser:appgroup /data

USER appuser

# Database path
VOLUME ["/data"]
ENV DB_PATH=/data/ratings.db

# Default to webhook mode (change to /app/stream for gRPC stream mode)
EXPOSE 8080
CMD ["/app/webhook"]
