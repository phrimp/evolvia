# Traefik Docker Labels Reference

## Basic Configuration
- `traefik.enable=true` - Enable Traefik for this container
- `traefik.http.routers.{name}.rule=PathPrefix(`/path`)` - Route based on path
- `traefik.http.services.{name}.loadbalancer.server.port=8080` - Container port
- `traefik.docker.network=traefik-net` - Specify the network

## Middlewares
- `traefik.http.routers.{name}.middlewares=rate-limit@consul,secure-headers@consul` - Apply middlewares

## TLS Configuration (HTTPS)
- `traefik.http.routers.{name}.tls=true` - Enable TLS
- `traefik.http.routers.{name}.tls.certresolver=letsencrypt` - Use Let's Encrypt
