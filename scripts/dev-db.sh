#!/bin/bash
# dev-db.sh — 在本机 MariaDB/MySQL 上建一个开发/测试库（绝不要对生产库执行）。
#
# 用法：
#   scripts/dev-db.sh <库名> [用户名] [密码]
#   例：scripts/dev-db.sh mistdocs_it mistdocs_dev mistdocs-local-dev
#
# 会 DROP 并重建 <库名>，然后：
#   1. 导入 docker/init-db.sql（全部 md_* 表结构，由 scripts/gen-init-db.sh 生成）
#   2. 导入 docker/dev-portal-tables.sql（Portal 共享表的最小结构 + 演示团队，仅供本地开发）
# 程序启动时 database.Migrate 仍会幂等地补齐缺失的表/列。
#
# 需要本机 root 访问（默认用 `sudo mariadb`，可用 MYSQL_ROOT="mysql -uroot -pxxx" 覆盖）。
set -euo pipefail

DB="${1:?用法: $0 <库名> [用户名] [密码]}"
USER_NAME="${2:-mistdocs_dev}"
USER_PASS="${3:-mistdocs-local-dev}"
ROOT_CMD="${MYSQL_ROOT:-sudo mariadb}"
HERE="$(cd "$(dirname "$0")/.." && pwd)"

case "$DB" in
  *prod*|mist_team) echo "!! 拒绝对疑似生产库名 '$DB' 执行" >&2; exit 1;;
esac

$ROOT_CMD <<SQL
DROP DATABASE IF EXISTS \`$DB\`;
CREATE DATABASE \`$DB\` CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci;
CREATE USER IF NOT EXISTS '$USER_NAME'@'127.0.0.1' IDENTIFIED BY '$USER_PASS';
CREATE USER IF NOT EXISTS '$USER_NAME'@'localhost' IDENTIFIED BY '$USER_PASS';
GRANT ALL PRIVILEGES ON \`$DB\`.* TO '$USER_NAME'@'127.0.0.1';
GRANT ALL PRIVILEGES ON \`$DB\`.* TO '$USER_NAME'@'localhost';
FLUSH PRIVILEGES;
SQL

$ROOT_CMD "$DB" < "$HERE/docker/init-db.sql"

$ROOT_CMD "$DB" < "$HERE/docker/dev-portal-tables.sql"

echo "[dev-db] $DB ready (user $USER_NAME @ 127.0.0.1:3306)"
