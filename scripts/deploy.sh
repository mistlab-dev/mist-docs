#!/bin/bash
# deploy.sh — 安全部署 mist-docs 到生产服务器
#
# 用法：
#   scripts/deploy.sh                 部署（先备份前端与 md_ 表，再同步前端、替换后端）
#   scripts/deploy.sh --backup-db     只备份数据库里的 md_ 表
#   scripts/deploy.sh --rollback-web  前端回滚到上一次部署前的备份
#   scripts/deploy.sh --help
#
# 安全特性（对齐已验证的生产部署方式）：
#   - 后端二进制：先传 /tmp/*.new，用 install 原子替换 + 备份 .bak + /healthz 健康检查，失败回滚
#   - 前端：rsync 前把线上 web/ 复制为 web.bak-<时间>，保留最近 3 份，可 --rollback-web
#   - 数据库：部署前 mysqldump 只导出 md_ 开头的表（Portal 的 users/teams 不归本服务管），
#     连接信息读服务器上的 $DB_DEFAULTS（MySQL option file，需要自行创建，权限 600）；
#     文件不存在时只提示、不中断
#   - 绝不覆盖生产配置（/etc/mistdocs/config.yaml）和生产 master key
#     （/var/www/mistdocs/secrets/master.key）——旧版脚本会上传本地开发密钥，极其危险
#   - 使用 systemctl restart（非 stop;start）

set -euo pipefail

usage() { sed -n '2,8p' "$0" | sed 's/^# \{0,1\}//'; }

MODE=deploy
case "${1:-}" in
  "") ;;
  --backup-db) MODE=backup-db ;;
  --rollback-web) MODE=rollback-web ;;
  -h|--help) usage; exit 0 ;;
  *) echo "未知参数：$1"; usage; exit 2 ;;
esac

# 目标主机经环境变量传入，避免把生产地址写进仓库
# 本地可将目标写入 scripts/deploy.env（已 gitignore）
if [ -z "${MISTDOCS_PROD:-}" ] && [ -f "$(dirname "$0")/deploy.env" ]; then
  . "$(dirname "$0")/deploy.env"
fi
PROD="${MISTDOCS_PROD:?请设置 MISTDOCS_PROD=user@host（或写入 scripts/deploy.env）}"
REMOTE_DIR="${MISTDOCS_REMOTE_DIR:-/var/www/mistdocs}"
WEB_DIR="$REMOTE_DIR/web"
SERVICE="mist-docs"
HEALTH_URL="http://127.0.0.1:8900/healthz"
BACKUP_DIR="${MISTDOCS_BACKUP_DIR:-/var/backups/mistdocs}"
DB_DEFAULTS="${MISTDOCS_DB_DEFAULTS:-/etc/mistdocs/backup.cnf}"
DB_NAME="${MISTDOCS_DB_NAME:-mist_team}"

backup_db() {
  echo "=== 备份数据库 md_ 表 ==="
  ssh "$PROD" bash -s <<EOF
set -euo pipefail
if [ ! -r "$DB_DEFAULTS" ]; then
  echo "!! 跳过数据库备份：服务器上没有 $DB_DEFAULTS"
  echo "   创建方法：[client] user=... password=... host=127.0.0.1，chmod 600"
  exit 0
fi
mkdir -p "$BACKUP_DIR"
tables=\$(mysql --defaults-extra-file="$DB_DEFAULTS" -N -B "$DB_NAME" -e "SHOW TABLES LIKE 'md\\\\_%'")
if [ -z "\$tables" ]; then echo "!! 没有找到 md_ 表，未备份"; exit 1; fi
out="$BACKUP_DIR/md-\$(date +%Y%m%d-%H%M%S).sql.gz"
mysqldump --defaults-extra-file="$DB_DEFAULTS" --single-transaction --quick --routines=false \
  "$DB_NAME" \$tables | gzip > "\$out"
chmod 600 "\$out"
echo "[+] 已备份 \$(echo \$tables | wc -w) 张表 -> \$out (\$(du -h "\$out" | cut -f1))"
ls -1t "$BACKUP_DIR"/md-*.sql.gz | tail -n +15 | xargs -r rm -f
EOF
}

backup_web() {
  echo "=== 备份线上前端 ==="
  ssh "$PROD" bash -s <<EOF
set -euo pipefail
if [ -d "$WEB_DIR" ]; then
  dst="$REMOTE_DIR/web.bak-\$(date +%Y%m%d-%H%M%S)"
  cp -a "$WEB_DIR" "\$dst"
  echo "[+] \$dst"
  ls -1dt "$REMOTE_DIR"/web.bak-* | tail -n +4 | xargs -r rm -rf
else
  echo "   线上还没有 $WEB_DIR，跳过"
fi
EOF
}

rollback_web() {
  echo "=== 前端回滚到最近一次备份 ==="
  ssh "$PROD" bash -s <<EOF
set -euo pipefail
latest=\$(ls -1dt "$REMOTE_DIR"/web.bak-* 2>/dev/null | head -1 || true)
if [ -z "\$latest" ]; then echo "!! 没有可用的前端备份"; exit 1; fi
rm -rf "$WEB_DIR.rollback-tmp"
cp -a "\$latest" "$WEB_DIR.rollback-tmp"
rm -rf "$WEB_DIR"
mv "$WEB_DIR.rollback-tmp" "$WEB_DIR"
echo "[+] 已恢复 \$latest -> $WEB_DIR"
EOF
}

case "$MODE" in
  backup-db) backup_db; exit 0 ;;
  rollback-web) rollback_web; exit 0 ;;
esac

test -d web/dist || { echo "!! web/dist 不存在，请先构建前端"; exit 1; }
test -f mist-docs-linux || { echo "!! mist-docs-linux 不存在，请先构建"; exit 1; }

echo "=== 0. 部署前备份 ==="
backup_db
backup_web

echo "=== 1. 同步前端 ==="
ssh "$PROD" "mkdir -p $WEB_DIR"
rsync -az --delete web/dist/ "$PROD:$WEB_DIR/"

echo "=== 2. 上传后端二进制（暂存 /tmp，不直接覆盖运行中文件）==="
scp mist-docs-linux "$PROD:/tmp/$SERVICE.new"

echo "=== 3. 原子替换 + 备份 + 健康检查 + 回滚 ==="
ssh "$PROD" bash -s <<EOF
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
  echo "   前端如需回滚：scripts/deploy.sh --rollback-web"
  exit 1
fi
EOF

echo "=== 4. 最终状态 ==="
ssh "$PROD" "systemctl is-active $SERVICE && curl -fsS $HEALTH_URL && echo && systemctl status $SERVICE --no-pager -l" | head -12

echo "=== 部署完成（生产配置与 master key 保持不变）==="
