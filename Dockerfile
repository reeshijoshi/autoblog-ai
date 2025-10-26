# Multi-stage build for AutoBlog AI
# Stage 1: Builder
FROM golang:1.25-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download
RUN go mod verify

# Copy source code
COPY . .

# Build the application
# CGO_ENABLED=0 for static binary
# -ldflags to strip debug info and set version
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s -X main.Version=${VERSION}" \
    -a -installsuffix cgo \
    -o autoblog-ai .

# Stage 2: Runtime
FROM alpine:3.21

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser

# Set working directory
WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/autoblog-ai .

# Copy configuration files and templates
COPY --chown=appuser:appuser config.yaml .
COPY --chown=appuser:appuser topics.csv .
COPY --chown=appuser:appuser templates/ ./templates/

# Create directory for generated articles
RUN mkdir -p /app/generated && chown appuser:appuser /app/generated

# Switch to non-root user
USER appuser

# Set environment variables
ENV TZ=UTC

# Health check (for Kubernetes liveness/readiness probes)
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD [ -f /app/autoblog-ai ] || exit 1

# Run the application
ENTRYPOINT ["/app/autoblog-ai"]

# Default to dry-run mode (override in docker-compose or k8s)
CMD ["--dry-run"]
