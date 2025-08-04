FROM golang:1.24.4-alpine

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache gcc musl-dev

# Copy go mod and sum files
COPY services/quiz-service/go.mod services/quiz-service/go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY services/quiz-service/ ./

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o main .

# Create final lightweight image
FROM alpine:latest

WORKDIR /app

# Copy binary from build stage
COPY --from=0 /app/main .

# Create log directory
RUN mkdir -p /evolvia/log/quiz_service

# Run the service
CMD ["./main"]