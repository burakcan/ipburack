# Build stage
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Install git for go mod download (some deps may need it)
RUN apk add --no-cache git

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /server ./cmd/server

# Run stage
FROM alpine:3.21

# Install ca-certificates for HTTPS requests
RUN apk add --no-cache ca-certificates

# Create non-root user
RUN addgroup -S app && adduser -S app -G app

# Create data directory
RUN mkdir -p /data && chown app:app /data

WORKDIR /app

# Copy binary from builder
COPY --from=builder /server .

# Switch to non-root user
USER app

# Default port (actual port set via PORT env var at runtime)
EXPOSE 3002

# Health check (uses shell to expand PORT env var)
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD sh -c 'wget --no-verbose --tries=1 --spider http://localhost:${PORT:-3002}/health || exit 1'

# Run the server
CMD ["./server"]
