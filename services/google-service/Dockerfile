# Stage 1: Build
FROM golang:1.24-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git

COPY services/google-service/go.mod services/google-service/go.sum ./

COPY proto-gen/ /proto-gen/

RUN go mod download

COPY services/google-service/ ./

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main ./cmd

#Stage 2
FROM alpine:3.18

WORKDIR /app
RUN apk add --no-cache tzdata
RUN apk --no-cache add ca-certificates && \
    addgroup -g 1001 appgroup && \
    adduser -D -u 1001 -G appgroup appuser
COPY --from=builder /app/main .

RUN chown -R appuser:appgroup /app

USER appuser

CMD ["./main"]
