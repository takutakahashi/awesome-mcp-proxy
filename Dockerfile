# Build stage
FROM golang:1.24-alpine AS builder

# Install ca-certificates for HTTPS requests
RUN apk add --no-cache ca-certificates git

# Set working directory
WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the binary with optimizations
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags='-w -s -extldflags "-static"' \
    -a -installsuffix cgo \
    -o awesome-mcp-proxy .

# Final stage
FROM scratch

# Import from builder
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy binary
COPY --from=builder /app/awesome-mcp-proxy /awesome-mcp-proxy

# Copy example configuration
COPY --from=builder /app/examples/ /examples/

# Expose default port
EXPOSE 8080

# Set entrypoint
ENTRYPOINT ["/awesome-mcp-proxy"]

# Default command
CMD ["-addr", ":8080"]