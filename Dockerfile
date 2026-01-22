# Build Stage
FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o github-exporter

# Run Stage
FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/github-exporter .
USER nobody:nogroup
CMD ["./github-exporter"]
