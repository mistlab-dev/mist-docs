#!/bin/sh
set -e

# ─── Wait for MySQL ───
DB_HOST="${DB_HOST:-${MYSQL_HOST:-mysql}}"
DB_PORT="${DB_PORT:-${MYSQL_PORT:-3306}}"
DB_USER="${DB_USER:-${MYSQL_USER:-mist_docs}}"
DB_PASS="${DB_PASS:-${MYSQL_PASSWORD:-mist_docs_pass}}"
DB_NAME="${DB_NAME:-${MYSQL_DATABASE:-mist_docs}}"

echo "⏳ Waiting for MySQL at ${DB_HOST}:${DB_PORT}..."

max_wait=60
waited=0
while ! mysqladmin ping -h"${DB_HOST}" -P"${DB_PORT}" -u"${DB_USER}" -p"${DB_PASS}" --silent 2>/dev/null; do
    sleep 1
    waited=$((waited + 1))
    if [ $waited -ge $max_wait ]; then
        echo "❌ MySQL not ready after ${max_wait}s, exiting"
        exit 1
    fi
done
echo "✅ MySQL is ready"

# ─── Generate config from env if no custom config ───
CONFIG_FILE="${CONFIG_FILE:-/app/configs/config.yaml}"

if [ "${GENERATE_CONFIG:-yes}" = "yes" ]; then
    echo "📝 Generating config from environment..."
    cat > "$CONFIG_FILE" << EOF
server:
  host: "0.0.0.0"
  port: ${SERVER_PORT:-8900}

database:
  host: "${DB_HOST}"
  port: ${DB_PORT}
  user: "${DB_USER}"
  password: "${DB_PASS}"
  dbname: "${DB_NAME}"
  max_open_conns: ${DB_MAX_OPEN:-20}
  max_idle_conns: ${DB_MAX_IDLE:-10}

jwt:
  secret: "${JWT_SECRET:-mist-docs-jwt-secret-change-me}"
  issuer: "${JWT_ISSUER:-mist-docs}"
  expire_hours: ${JWT_EXPIRE_HOURS:-24}

storage:
  root: "${DATA_DIR:-/data}/files"
  max_file_size: ${MAX_FILE_SIZE:-52428800}
  version_keep: ${VERSION_KEEP:-20}

websocket:
  max_message_size: ${WS_MAX_MSG:-1048576}
  ping_interval: ${WS_PING_INTERVAL:-30}

log:
  level: "${LOG_LEVEL:-info}"
EOF
fi

echo "🚀 Starting MistDocs..."
exec "$@"
