FROM python:3.11-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache \
    gcc \
    musl-dev \
    libffi-dev \
    python3-dev

# Copy requirements and install Python dependencies
COPY services/input-service/requirements.txt .
RUN pip install --no-cache-dir --user -r requirements.txt

# Stage 2: Runtime
FROM alpine:3.18

WORKDIR /app

# Install runtime dependencies
RUN apk add --no-cache \
    python3 \
    py3-pip \
    tzdata \
    ca-certificates && \
    addgroup -g 1001 appgroup && \
    adduser -D -u 1001 -G appgroup appuser

# Copy Python installation from builder
COPY --from=builder /root/.local /home/appuser/.local

# Copy application code
COPY services/input-service/ .

# Create log directory and set permissions
RUN mkdir -p /evolvia/log/input_service && \
    mkdir -p /tmp/uploads && \
    chown -R appuser:appgroup /app /evolvia /tmp/uploads

# Set PATH for user-installed packages
ENV PATH=/home/appuser/.local/bin:$PATH

USER appuser

CMD ["python", "main.py"]
