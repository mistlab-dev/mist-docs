# ─── Stage 1: Build backend ───
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /mist-docs ./cmd/server/

# ─── Stage 2: Build frontend ───
FROM node:20-alpine AS frontend

WORKDIR /app
COPY web/package.json web/pnpm-lock.yaml* web/yarn.lock* web/package-lock.json* ./
RUN npm install --prefer-offline 2>/dev/null || true
COPY web/ .
RUN npm run build

# ─── Stage 3: Runtime ───
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata mysql-client
RUN cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime && echo "Asia/Shanghai" > /etc/timezone

WORKDIR /app

# Binary
COPY --from=builder /mist-docs /usr/local/bin/mist-docs

# Frontend static files
COPY --from=frontend /app/dist /app/web/dist

# Default config
COPY configs/config.yaml /app/configs/config.yaml

# Entrypoint
COPY docker-entrypoint.sh /app/docker-entrypoint.sh
RUN chmod +x /app/docker-entrypoint.sh

# Data directories
RUN mkdir -p /data/files /data/uploads

ENV CONFIG_FILE=/app/configs/config.yaml
ENV DATA_DIR=/data

EXPOSE 8900

VOLUME ["/data"]

ENTRYPOINT ["/app/docker-entrypoint.sh"]
CMD ["mist-docs"]
