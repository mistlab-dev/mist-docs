#!/bin/bash
# deploy.sh — 安全部署 mist-docs 到生产服务器
#
# 安全特性（对齐已验证的生产部署方式）：
#   - 后端二进制：先传 /tmp/*.new，用 install 原子替换 + 备份 .bak + /healthz 健康检查，失败回滚
#   - 绝不覆盖生产配置（/etc/mistdocs/config.yaml）和生产 master key
#     （/var/www/mistdocs/secrets/master.key）——旧版脚本会上传本地开发密钥，极其危险
#   - 只同步前端 dist + 安全替换后端二进制
#   - 使用 systemctl restart（非 stop;start）

set -e

PROD="root@85.137.247.166"
REMOTE_DIR="/var/www/mistdocs"
WEB_DIR="$REMOTE_DIR/web"
SERVICE="mist-docs"
HEALTH_URL="http://127.0.0.1:8900/healthz"

echo "=== 1. 同步前端 ==="
ssh "$PROD" "mkdir -p $WEB_DIR"
rsync -az --delete web/dist/ "$PROD:$WEB_DIR/"

echo "=== 2. 上传后端二进制（暂存 /tmp，不直接覆盖运行中文件）==="
test -f mist-docs-linux || { echo "!! mist-docs-linux 不存在，请先构建"; exit 1; }
scp mist-docs-linux "$PROD:/tmp/$SERVICE.new"

echo "=== 3. 原子替换 + 备份 + 健康检查 + 回滚 ==="
ssh "$PROD" "$SHELL" <<EOF
set -e
test -s /tmp/$SERVICE.new || { echo "!! 上传的二进制为空"; exit 1; }
chmod 755 /tmp/$SERVICE.new
test -f /usr/local/bin/$SERVICE && cp -p /usr/local/bin/$SERVICE /usr/local/bin/$SERVICE.bak || true
install -o root -g root -m 755 /tmp/$SERVICE.new /usr/local/bin/$SERVICE
rm -f /tmp/$SERVICE.new
systemctl restart $SERVICE
sleep 3
ok=0
if systemctl is-active --quiet $SERVICE; then
  if curl -fsS -o /dev/null $HEALTH_URL; then
    echo "[+] /healthz OK"
    ok=1
  else
    echo "!! /healthz failed after deploy"
  fi
else
  echo "!! service failed to start"
fi
if [ "\$ok" != "1" ]; then
  echo "!! rolling back to previous binary..."
  if [ -f /usr/local/bin/$SERVICE.bak ]; then
    install -o root -g root -m 755 /usr/local/bin/$SERVICE.bak /usr/local/bin/$SERVICE
    systemctl restart $SERVICE
    sleep 3
  fi
  exit 1
fi
EOF

echo "=== 4. 最终状态 ==="
ssh "$PROD" "systemctl is-active $SERVICE && curl -fsS $HEALTH_URL && echo && systemctl status $SERVICE --no-pager -l" | head -12

echo "=== 部署完成（生产配置与 master key 保持不变）==="
