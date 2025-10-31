FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install git (required for go mod download)
RUN apk add --no-cache git

# Copy source code
COPY . .

# Download dependencies and build the application
RUN go mod tidy && \
    CGO_ENABLED=0 GOOS=linux go build -o /mcp-server .

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy the binary from builder
COPY --from=builder /mcp-server .

# Run the server
ENTRYPOINT ["./mcp-server"]
