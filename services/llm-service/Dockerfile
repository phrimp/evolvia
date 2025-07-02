# Build stage
FROM golang:1.24-alpine AS builder

# Set working directory
WORKDIR /app

# Install git and ca-certificates (needed for go mod download)
RUN apk add --no-cache git ca-certificates tzdata

# Copy go mod files first for better caching
COPY services/llm-service/go.mod services/llm-service/go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY services/llm-service/ .

# Build the application with optimizations
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags='-w -s -extldflags "-static"' \
    -a -installsuffix cgo \
    -o llm-service main.go

# Final stage
FROM alpine:latest

# Install ca-certificates for HTTPS requests and timezone data
RUN apk --no-cache add ca-certificates tzdata wget && \
    update-ca-certificates

# Create non-root user for security
RUN addgroup -g 1001 -S appgroup && \
    adduser -u 1001 -S appuser -G appgroup

# Set working directory
WORKDIR /app

# Copy the binary from builder stage
COPY --from=builder /app/llm-service .

# Copy RAG files
COPY --from=builder /app/rag ./rag

# Change ownership to non-root user
RUN chown -R appuser:appgroup /app

# Switch to non-root user
USER appuser

# Run the application
CMD ["./llm-service"]