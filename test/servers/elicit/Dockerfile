# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Copy go mod files and vendor directory
COPY go.mod go.sum ./
COPY vendor/ ./vendor/

# Copy source code
COPY test/servers/elicit/server.go ./server.go

# Build the binary using vendor directory
RUN CGO_ENABLED=0 GOOS=linux go build -mod=vendor -a -installsuffix cgo -o server server.go

# Runtime stage  
FROM alpine:latest

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy the binary from builder stage
COPY --from=builder /app/server .

LABEL io.modelcontextprotocol.server.name="io.github.slimslenderslacks/poci"

# Run the binary
ENTRYPOINT ["./server"]
