FROM oven/bun

WORKDIR /app

# Copy package files from the service directory
COPY services/payos-service/package.json .
COPY services/payos-service/bun.lock* .

RUN bun install --production

# Copy source code and config from the service directory
COPY services/payos-service/src src
COPY services/payos-service/tsconfig.json .

ENV NODE_ENV production
CMD ["bun", "src/index.ts"]
