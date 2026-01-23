# Build Stage
FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./

RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build \
    -a -installsuffix cgo \
    -ldflags="-w -s" \
    -o github-exporter

# Run Stage - using pinned alpine version
FROM alpine:3.23


# Install CA certificates (required for HTTPS calls to GitHub API)
RUN apk add --no-cache ca-certificates

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/github-exporter .

# Add OCI labels for metadata
LABEL org.opencontainers.image.title="github-exporter" \
      org.opencontainers.image.description="Prometheus exporter for GitHub metrics" \
      org.opencontainers.image.source="https://github.com/eleboucher/github-exporter" \
      org.opencontainers.image.licenses="APACHE-2.0" \

# Run as non-root user
USER nobody:nogroup

ENTRYPOINT ["/app/github-exporter"]

CMD ["--config", "/config/config.yaml"]
