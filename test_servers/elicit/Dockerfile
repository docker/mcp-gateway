# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Copy go mod files and vendor directory
COPY go.mod go.sum ./
COPY vendor/ ./vendor/

# Copy source code
COPY server.go ./

# Add main function to make it executable
RUN echo -e "\n\nfunc main() {\n\tserver()\n}" >> server.go

# Build the binary using vendor directory
RUN CGO_ENABLED=0 GOOS=linux go build -mod=vendor -a -installsuffix cgo -o server server.go

# Runtime stage  
FROM alpine:latest

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy the binary from builder stage
COPY --from=builder /app/server .

# Run the binary
ENTRYPOINT ["./server"]
