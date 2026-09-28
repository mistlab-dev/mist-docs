#!/usr/bin/env bash
# backup-md-tables.sh — 一致性备份 MistDocs 的全部 md_* 表（含旧版遗留表）
#
# 用 mysqldump --single-transaction 做 InnoDB 一致性快照，不锁表，线上可跑。
# 只导出 md_ 前缀的表；Portal 共享的 users/teams/team_members/fragments 不在内
# （它们属于 Portal，由 Portal 那边备份）。
#
# 用法：
#   scripts/backup-md-tables.sh <库名> [输出目录]
#
# 连接参数通过 MySQL 选项文件传入，避免密码出现在命令行/进程列表里：
#   MYSQL_DEFAULTS=~/.my-mistdocs.cnf scripts/backup-md-tables.sh mist_docs /var/backups/mistdocs
# 选项文件示例（chmod 600）：
#   [client]
#   host=127.0.0.1
#   user=...
#   password=...
#
# 产物：<输出目录>/md-tables-<库名>-<时间>.sql.gz 以及同名 .sha256。
# 脚本结束前会用 gzip -t 校验压缩包，并列出导出的表。
#
# 恢复（到一个新库里先核对，确认无误再考虑覆盖）：
#   gunzip -c md-tables-....sql.gz | mysql --defaults-extra-file=... <新库名>
set -euo pipefail

DB="${1:?用法: $0 <库名> [输出目录]}"
OUT_DIR="${2:-.}"
DEFAULTS="${MYSQL_DEFAULTS:-}"

conn=()
if [[ -n "$DEFAULTS" ]]; then
  [[ -r "$DEFAULTS" ]] || { echo "!! 读不到选项文件 $DEFAULTS" >&2; exit 1; }
  conn=(--defaults-extra-file="$DEFAULTS")
fi

MYSQL="${MYSQL_BIN:-mysql}"
MYSQLDUMP="${MYSQLDUMP_BIN:-mysqldump}"
command -v "$MYSQLDUMP" >/dev/null || { echo "!! 找不到 $MYSQLDUMP" >&2; exit 1; }

mapfile -t tables < <("$MYSQL" "${conn[@]}" -N -B -e \
  "SELECT TABLE_NAME FROM information_schema.TABLES
   WHERE TABLE_SCHEMA = '$(printf '%s' "$DB" | sed "s/'/''/g")'
     AND TABLE_TYPE = 'BASE TABLE'
     AND (TABLE_NAME LIKE 'md\\_%' OR TABLE_NAME LIKE '\\_archived\\_md\\_%')
   ORDER BY TABLE_NAME")

if [[ ${#tables[@]} -eq 0 ]]; then
  echo "!! 库 $DB 里没有 md_* 表，检查库名/权限" >&2
  exit 1
fi

mkdir -p "$OUT_DIR"
stamp="$(date +%Y%m%d-%H%M%S)"
out="$OUT_DIR/md-tables-$DB-$stamp.sql.gz"
umask 077

echo "[backup] 库 $DB，${#tables[@]} 张表 → $out"
printf '  %s\n' "${tables[@]}"

"$MYSQLDUMP" "${conn[@]}" \
  --single-transaction --quick --skip-lock-tables \
  --default-character-set=utf8mb4 --hex-blob \
  --no-tablespaces --triggers \
  "$DB" "${tables[@]}" | gzip -c > "$out"

gzip -t "$out"
( cd "$(dirname "$out")" && sha256sum "$(basename "$out")" > "$(basename "$out").sha256" )
echo "[backup] 完成：$(du -h "$out" | cut -f1)  sha256: $(cut -d' ' -f1 "$out.sha256")"
