# Production Dockerfile for DeploySentry API
# Multi-stage build for security and size optimization

# Build stage
FROM golang:1.21-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Create non-root user for build
RUN addgroup -g 1001 deploysentry && \
    adduser -D -s /bin/sh -u 1001 -G deploysentry deploysentry

WORKDIR /app

# Copy dependency files and download dependencies first (for better caching)
COPY go.mod go.sum ./
RUN go mod download && go mod verify

# Copy source code
COPY . .

# Build the application with optimizations
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags='-w -s -extldflags "-static"' \
    -a -installsuffix cgo \
    -o deploysentry-api \
    ./cmd/api

# Production stage
FROM scratch

# Copy CA certificates for HTTPS requests
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy timezone data
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# Copy user information
COPY --from=builder /etc/passwd /etc/passwd
COPY --from=builder /etc/group /etc/group

# Copy the binary
COPY --from=builder /app/deploysentry-api /deploysentry-api

# Run as non-root user
USER deploysentry:deploysentry

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD ["/deploysentry-api", "health-check"] || exit 1

# Run the application
ENTRYPOINT ["/deploysentry-api"]