# Build Stage
FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY main.go .
RUN CGO_ENABLED=0 GOOS=linux go build -o exporter main.go

# Run Stage
FROM alpine:latest
WORKDIR /root/
COPY --from=builder /app/exporter .
EXPOSE 2112
CMD ["./exporter"]
