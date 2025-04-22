#!/bin/bash

# Script to register Traefik's dynamic configuration in Consul directly from environment variables
# Usage: ./register-traefik-config-to-consul.sh

echo "Waiting for Consul to be ready..."
until curl -s http://consul-server:8500/v1/status/leader | grep -q .; do
  sleep 5
done
echo "Consul is ready."

# Rate limiting middleware
echo "Registering rate-limit middleware..."
AVERAGE=${TRAEFIK_RATE_LIMIT_AVERAGE:-100}
BURST=${TRAEFIK_RATE_LIMIT_BURST:-50}

curl -X PUT -d '{
  "rateLimit": {
    "average": '"$AVERAGE"',
    "burst": '"$BURST"'
  }
}' http://consul-server:8500/v1/kv/traefik/http/middlewares/rate-limit/

echo "Rate limit middleware registered successfully"

# Secure headers middleware
echo "Registering secure-headers middleware..."
curl -X PUT -d '{
  "headers": {
    "frameDeny": true,
    "browserXssFilter": true,
    "contentTypeNosniff": true,
    "stsSeconds": 31536000,
    "stsIncludeSubdomains": true
  }
}' http://consul-server:8500/v1/kv/traefik/http/middlewares/secure-headers/

echo "Secure headers middleware registered successfully"

# Compression middleware
echo "Registering compress middleware..."
curl -X PUT -d '{
  "compress": {
    "excludedContentTypes": [
      "text/event-stream"
    ]
  }
}' http://consul-server:8500/v1/kv/traefik/http/middlewares/compress/

echo "Compression middleware registered successfully"

# IP whitelist middleware
if [ "${TRAEFIK_IP_WHITELIST_ENABLED:-false}" = "true" ]; then
  echo "Registering ipwhitelist middleware..."

  # Convert comma-separated IPs to JSON array
  IPS=$(echo ${TRAEFIK_IP_WHITELIST} | tr ',' ' ')
  JSON_IPS="["
  for ip in $IPS; do
    JSON_IPS+="\"$ip\","
  done
  JSON_IPS=${JSON_IPS%,} # Remove trailing comma
  JSON_IPS+="]"

  curl -X PUT -d '{
    "ipWhiteList": {
      "sourceRange": '"$JSON_IPS"'
    }
  }' http://consul-server:8500/v1/kv/traefik/http/middlewares/ipwhitelist/

  echo "IP whitelist middleware registered successfully"
fi

echo "Traefik configuration has been successfully registered in Consul."
