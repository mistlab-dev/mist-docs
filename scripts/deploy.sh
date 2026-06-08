#!/bin/bash
set -e
PROD="root@85.137.247.166"
REMOTE_DIR="/var/www/mistdocs"

WEB_DIR="$REMOTE_DIR/web"

echo "=== 1. 同步前端 ==="
ssh "$PROD" "mkdir -p $WEB_DIR"
rsync -az --delete web/dist/ "$PROD:$WEB_DIR/"

echo "=== 2. 上传二进制 ==="
scp mist-docs-linux "$PROD:/usr/local/bin/mist-docs"

echo "=== 3. 上传配置 ==="
scp configs/config.yaml "$PROD:$REMOTE_DIR/config.yaml"

echo "=== 4. 上传 master key ==="
ssh "$PROD" "mkdir -p $REMOTE_DIR/secrets"
scp secrets/master.key "$PROD:$REMOTE_DIR/secrets/master.key"
ssh "$PROD" "chmod 600 $REMOTE_DIR/secrets/master.key"

echo "=== 5. 重启服务 ==="
ssh "$PROD" "chmod +x /usr/local/bin/mist-docs && systemctl stop mist-docs; systemctl start mist-docs"

echo "=== 部署完成 ==="
