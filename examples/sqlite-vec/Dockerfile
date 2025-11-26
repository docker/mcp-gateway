# Build stage
FROM golang:1.24-alpine AS builder

# Install build dependencies
RUN apk add --no-cache gcc musl-dev sqlite-dev

WORKDIR /build

# Copy go mod files
COPY go.mod go.sum* ./
RUN go mod download

# Copy source code
COPY main.go ./

# Build the application with sqlite extension support
RUN CGO_ENABLED=1 go build -tags "sqlite_extensions" -o vector-server -ldflags="-s -w" main.go

# Runtime stage
FROM alpine:latest

# Install runtime dependencies
RUN apk add --no-cache sqlite-libs ca-certificates wget

# Download and install sqlite-vec extension (x86_64 only)
WORKDIR /tmp
RUN wget https://github.com/asg017/sqlite-vec/releases/download/v0.1.1/sqlite-vec-0.1.1-loadable-linux-x86_64.tar.gz && \
    tar xzf sqlite-vec-0.1.1-loadable-linux-x86_64.tar.gz && \
    mkdir -p /usr/local/lib/sqlite && \
    mv vec0.so /usr/local/lib/sqlite/ && \
    rm -rf /tmp/*

# Copy binary from builder
COPY --from=builder /build/vector-server /usr/local/bin/

# Create data directory
RUN mkdir -p /data

# Set environment variables
ENV DB_PATH=/data/vectors.db
ENV VECTOR_DIMENSION=1536

WORKDIR /data

# Run as MCP stdio server
CMD ["vector-server"]
