# Multi-stage build for ChronosDB
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git ca-certificates

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build ChronosDB (disable CGO for static binary)
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o chronosd ./cmd/chronosd

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /root/

# Copy binary from builder
COPY --from=builder /app/chronosd .
COPY --from=builder /app/data ./data

# Expose ports
EXPOSE 8080 50051

# Run ChronosDB
ENTRYPOINT ["./chronosd"]
CMD ["-rest-port=8080", "-grpc-port=50051", "-data-dir=/root/data"]
