#!/bin/bash
set -e
PROD="root@85.137.247.166"
REMOTE_DIR="/var/www/mistdocs"

echo "=== 1. 同步前端 ==="
rsync -az --delete web/dist/ "$PROD:$REMOTE_DIR/web/"

echo "=== 2. 上传二进制 ==="
scp mist-docs-linux "$PROD:$REMOTE_DIR/mist-docs"

echo "=== 3. 上传配置 ==="
scp configs/config.yaml "$PROD:$REMOTE_DIR/config.yaml"

echo "=== 4. 上传 master key ==="
ssh "$PROD" "mkdir -p $REMOTE_DIR/secrets"
scp secrets/master.key "$PROD:$REMOTE_DIR/secrets/master.key"
ssh "$PROD" "chmod 600 $REMOTE_DIR/secrets/master.key"

echo "=== 5. 重启服务 ==="
ssh "$PROD" "cd $REMOTE_DIR && chmod +x mist-docs && sudo systemctl restart mist-docs 2>/dev/null || (echo 'Setting up systemd...' && sudo tee /etc/systemd/system/mist-docs.service > /dev/null && sudo systemctl daemon-reload && sudo systemctl enable mist-docs && sudo systemctl start mist-docs)"

echo "=== 部署完成 ==="
