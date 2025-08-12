# Build stage
FROM golang:1.23-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
ARG VERSION=dev
ARG COMMIT=unknown
ARG DATE=unknown

RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.date=${DATE}" \
    -o gotunnel ./cmd/gotunnel

# Final stage
FROM alpine:3.19

# Install runtime dependencies
RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    curl \
    bind-tools \
    && rm -rf /var/cache/apk/*

# Create non-root user
RUN addgroup -g 1000 gotunnel && \
    adduser -D -s /bin/sh -u 1000 -G gotunnel gotunnel

# Set working directory
WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/gotunnel .

# Copy configuration files
COPY --chown=gotunnel:gotunnel configs/ ./configs/

# Create directories with proper permissions
RUN mkdir -p /app/certs /app/data && \
    chown -R gotunnel:gotunnel /app

# Switch to non-root user
USER gotunnel

# Expose default ports
EXPOSE 8080 8443

# Health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:8080/health || exit 1

# Default command
ENTRYPOINT ["./gotunnel"]
CMD ["--help"]