# Build Stage
FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o github-exporter

# Run Stage
FROM alpine:latest
WORKDIR /root/
COPY --from=builder /app/github-exporter .
EXPOSE 2112
CMD ["./github-exporter"]
