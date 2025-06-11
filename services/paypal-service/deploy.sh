#!/bin/bash

# PayPal Service Deployment Script
set -e

echo "🚀 Starting PayPal Service Deployment..."

# Load environment variables
if [ -f .env ]; then
    export $(cat .env | xargs)
fi

# Check required environment variables
required_vars=("PAYPAL_CLIENT_ID" "PAYPAL_CLIENT_SECRET" "PAYPAL_MODE")
for var in "${required_vars[@]}"; do
    if [ -z "${!var}" ]; then
        echo "❌ Error: $var is not set"
        exit 1
    fi
done

echo "✅ Environment variables validated"

# Build the application
echo "📦 Building application..."
bun run build

# Run tests if they exist
if [ -d "test" ] || [ -f "bun.test.ts" ]; then
    echo "🧪 Running tests..."
    bun test
fi

# Deploy based on environment
if [ "$PAYPAL_MODE" = "production" ]; then
    echo "🔴 Deploying to PRODUCTION environment"
    
    # Production deployment steps
    echo "Building Docker image..."
    docker build -t paypal-service:latest .
    
    echo "Stopping existing container..."
    docker stop paypal-service || true
    docker rm paypal-service || true
    
    echo "Starting new container..."
    docker run -d \
        --name paypal-service \
        --restart unless-stopped \
        -p 3000:3000 \
        --env-file .env \
        paypal-service:latest
        
    echo "Waiting for service to start..."
    sleep 10
    
    # Health check
    if curl -f http://localhost:3000/api/paypal/health > /dev/null 2>&1; then
        echo "✅ Production deployment successful!"
        echo "🌐 Service running at: http://localhost:3000"
    else
        echo "❌ Health check failed"
        exit 1
    fi
    
else
    echo "🟡 Deploying to SANDBOX environment"
    
    # Development deployment
    echo "Starting development server..."
    bun run start &
    SERVER_PID=$!
    
    echo "Waiting for service to start..."
    sleep 5
    
    # Health check
    if curl -f http://localhost:3000/api/paypal/health > /dev/null 2>&1; then
        echo "✅ Sandbox deployment successful!"
        echo "🌐 Service running at: http://localhost:3000"
        echo "📚 API Docs: http://localhost:3000/api"
        echo "🏥 Health Check: http://localhost:3000/api/paypal/health"
        echo "🧪 Test Client: Open test-client.html in your browser"
    else
        echo "❌ Health check failed"
        kill $SERVER_PID || true
        exit 1
    fi
fi

echo "🎉 Deployment completed successfully!"
