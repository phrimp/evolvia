FROM python:3.11-alpine AS builder

WORKDIR /build

# Install build dependencies
RUN apk add --no-cache \
    gcc \
    musl-dev \
    libffi-dev \
    python3-dev

# Copy requirements and install Python dependencies to a specific directory
COPY services/input-service/requirements.txt .
RUN pip install --no-cache-dir --prefix=/install -r requirements.txt

# Stage 2: Runtime
FROM python:3.11-alpine

WORKDIR /app

# Install runtime dependencies
RUN apk add --no-cache \
    tzdata \
    ca-certificates && \
    addgroup -g 1001 appgroup && \
    adduser -D -u 1001 -G appgroup appuser

# Copy Python packages from builder
COPY --from=builder /install /usr/local

# Copy application code from the app subdirectory
COPY services/input-service/app/ .

# Create log directory and set permissions
RUN mkdir -p /evolvia/log/input_service && \
    mkdir -p /tmp/uploads && \
    chown -R appuser:appgroup /app /evolvia /tmp/uploads

USER appuser

CMD ["python", "main.py"]
