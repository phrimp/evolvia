# Stage 1: Build
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git

# Copy and download dependencies first (better caching)
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main ./cmd

# Stage 2: Production
FROM alpine:3.18

WORKDIR /app
#Update TZ data
RUN apk add --no-cache tzdata
# Add CA certificates and create non-root user
RUN apk --no-cache add ca-certificates && \
    addgroup -g 1001 appgroup && \
    adduser -D -u 1001 -G appgroup appuser
# Copy the binary from builder
COPY --from=builder /app/main .

# Set ownership
RUN chown -R appuser:appgroup /app

# Use non-root user
USER appuser

# Run the binary
CMD ["./main"]
