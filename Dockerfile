# Multi-stage build
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Install dependencies
RUN apk add --no-cache git

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o chronosd ./cmd/chronosd
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o webserver ./cmd/webserver

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /app

# Copy binaries
COPY --from=builder /app/chronosd .
COPY --from=builder /app/webserver .
COPY --from=builder /app/webui ./webui

# Expose ports
EXPOSE 8080 8081 50051

# Start both services
CMD ./chronosd -rest-port=8080 -grpc-port=50051 -data-dir=/data & ./webserver
