#!/bin/bash
# dev-db.sh — 在本机 MariaDB/MySQL 上建一个开发/测试库（绝不要对生产库执行）。
#
# 用法：
#   scripts/dev-db.sh <库名> [用户名] [密码]
#   例：scripts/dev-db.sh mistdocs_it mistdocs_dev mistdocs-local-dev
#
# 会 DROP 并重建 <库名>，然后：
#   1. 导入 docker/init-db.sql（md_* 表结构）
#   2. 补齐团队化所需的列（与生产历次迁移一致）
#   3. 建 Portal 共享表的最小结构（users / teams / team_members / fragments），仅供本地开发
# 其余表（模板、团队文件夹、交期等）由程序启动时的 database.Migrate 自动创建。
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

if [ -f "$HERE/docker/dev-portal-tables.sql" ]; then
  $ROOT_CMD "$DB" < "$HERE/docker/dev-portal-tables.sql"
fi

# 老版 init-db.sql 缺少的团队化改动（新版 init-db.sql 已包含时，这些语句会被 --force 忽略）
$ROOT_CMD --force "$DB" 2>/dev/null <<'SQL' || true
ALTER TABLE md_documents DROP FOREIGN KEY fk_md_doc_dept;
ALTER TABLE md_documents DROP FOREIGN KEY fk_md_doc_folder;
ALTER TABLE md_documents MODIFY department_id VARCHAR(36) NOT NULL DEFAULT '';
ALTER TABLE md_documents ADD COLUMN team_id VARCHAR(64) DEFAULT '' AFTER id;
ALTER TABLE md_documents ADD INDEX idx_md_docs_team (team_id);
ALTER TABLE md_folders ADD COLUMN team_id VARCHAR(64) DEFAULT '' AFTER id;
ALTER TABLE md_audits ADD COLUMN team_id VARCHAR(64) DEFAULT '';
ALTER TABLE md_shares ADD COLUMN team_id VARCHAR(64) DEFAULT '';
ALTER TABLE md_comments ADD COLUMN team_id VARCHAR(64) DEFAULT '';
ALTER TABLE md_notifications ADD COLUMN team_id VARCHAR(64) DEFAULT '';
ALTER TABLE md_tags ADD COLUMN team_id VARCHAR(64) DEFAULT '';
CREATE TABLE IF NOT EXISTS users (
  id VARCHAR(64) PRIMARY KEY, email VARCHAR(255) DEFAULT '', username VARCHAR(100) DEFAULT '',
  display_name VARCHAR(100) DEFAULT '', password_hash VARCHAR(255) DEFAULT '',
  is_admin TINYINT(1) DEFAULT 0, email_verified TINYINT(1) DEFAULT 1
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
CREATE TABLE IF NOT EXISTS teams (
  id VARCHAR(64) PRIMARY KEY, name VARCHAR(200) NOT NULL, description VARCHAR(500) DEFAULT ''
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
CREATE TABLE IF NOT EXISTS team_members (
  team_id VARCHAR(64) NOT NULL, user_id VARCHAR(64) NOT NULL, role VARCHAR(32) NOT NULL,
  PRIMARY KEY (team_id, user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
CREATE TABLE IF NOT EXISTS fragments (
  id VARCHAR(64) PRIMARY KEY, team_id VARCHAR(64) NOT NULL, title VARCHAR(200) NOT NULL,
  command TEXT, category VARCHAR(100) DEFAULT '', status VARCHAR(32) DEFAULT 'published', deleted TINYINT DEFAULT 0
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
SQL

echo "[dev-db] $DB ready (user $USER_NAME @ 127.0.0.1:3306)"
