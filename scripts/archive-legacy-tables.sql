-- archive-legacy-tables.sql — 归档旧版遗留表 md_users / md_departments / md_folders
--
-- 这三张表来自团队化之前的 MistDocs（部门 + 本地账号），现在的代码已经不读它们
-- （团队文件夹是 md_team_folders，账号在 Portal 共享的 users 表）。
-- 新安装不会创建它们；只有老库里还有。
--
-- 做法（D5）：先备份，再“改名归档”，观察一段时间没问题再删除。改名可以秒级回滚。
--
-- !! 这个文件默认只执行第 1 节（只读检查）。第 2–4 节全部是注释，需要人工逐段
-- !! 取消注释后执行。不要整文件对生产库直接跑破坏性语句。
--
-- 建议顺序：
--   0. scripts/backup-md-tables.sh 备份所有 md_* 表（含这三张），确认备份文件可读
--   1. 执行本文件（只读），核对输出
--   2. 维护窗口内：把 @d 改成当天日期，取消第 2 节注释执行（改名）
--   3. 观察至少一周：应用日志、文档打开/保存、团队文件夹、审计页都正常
--      有问题 → 第 3 节回滚（改回原名）
--   4. 确认无影响后：第 4 节 DROP 归档表（不可逆，删除前再备份一次）
--
-- 用法（示例；连接参数用 --defaults-extra-file，别把密码写在命令行里）：
--   mysql --defaults-extra-file=~/.my-mistdocs.cnf <库名> < scripts/archive-legacy-tables.sql

-- ============================================================
-- 第 1 节：只读检查（默认执行）
-- ============================================================

SELECT DATABASE() AS current_database;

-- 1.1 三张表是否存在、各有多少行
SELECT TABLE_NAME, TABLE_ROWS AS approx_rows, ENGINE, CREATE_TIME
FROM information_schema.TABLES
WHERE TABLE_SCHEMA = DATABASE()
  AND TABLE_NAME IN ('md_users', 'md_departments', 'md_folders')
ORDER BY TABLE_NAME;

-- 1.2 有没有外键指向它们（有的话先处理外键，再改名）
SELECT TABLE_NAME, CONSTRAINT_NAME, COLUMN_NAME, REFERENCED_TABLE_NAME, REFERENCED_COLUMN_NAME
FROM information_schema.KEY_COLUMN_USAGE
WHERE TABLE_SCHEMA = DATABASE()
  AND REFERENCED_TABLE_NAME IN ('md_users', 'md_departments', 'md_folders');

-- 1.3 视图/触发器里有没有引用它们
SELECT TABLE_NAME AS view_name
FROM information_schema.VIEWS
WHERE TABLE_SCHEMA = DATABASE()
  AND (VIEW_DEFINITION LIKE '%md_users%' OR VIEW_DEFINITION LIKE '%md_departments%' OR VIEW_DEFINITION LIKE '%md_folders%');
SELECT TRIGGER_NAME, EVENT_OBJECT_TABLE
FROM information_schema.TRIGGERS
WHERE TRIGGER_SCHEMA = DATABASE()
  AND (EVENT_OBJECT_TABLE IN ('md_users', 'md_departments', 'md_folders')
       OR ACTION_STATEMENT LIKE '%md_users%' OR ACTION_STATEMENT LIKE '%md_departments%' OR ACTION_STATEMENT LIKE '%md_folders%');

-- （id 比较用 BINARY：Portal 表和 md_ 表的排序规则可能不同，直接 = 会报 1267。）

-- 1.4 仍挂在旧文件夹上的文档（folder_id 不在 md_team_folders 里）。
--     这些文档在团队界面里会显示在根目录；改名不影响它们的内容。
SELECT COUNT(*) AS docs_with_non_team_folder
FROM md_documents d
WHERE d.folder_id IS NOT NULL AND d.folder_id <> ''
  AND NOT EXISTS (SELECT 1 FROM md_team_folders tf WHERE BINARY tf.id = BINARY d.folder_id);

-- 1.5 没有归属团队的文档（团队界面本来就看不到它们，仅供知情）
SELECT COUNT(*) AS docs_without_team FROM md_documents WHERE team_id IS NULL OR team_id = '';

-- 1.6 创建人/更新人不在共享 users 表里的文档（多半是旧 md_users 账号；
--     显示名会退回为 id，改名表不会让情况变差）
SELECT COUNT(*) AS docs_with_unknown_creator
FROM md_documents d
WHERE d.created_by <> '' AND NOT EXISTS (SELECT 1 FROM users u WHERE BINARY u.id = BINARY d.created_by);

-- 注意：md_documents.department_id 仍被当作文件存储目录名使用（旧文档的
-- 版本文件在 <storage_root>/<department_id>/<doc_id>/ 下）。代码只用这个字符串，
-- 不查 md_departments，所以改名/删除 md_departments 不影响读取文件。

-- ============================================================
-- 第 2 节：改名归档（人工取消注释执行；执行前把日期改成当天）
-- ============================================================
-- RENAME TABLE 是原子的，三张表要么全部改名，要么都不改。表不存在会报错，
-- 那就把不存在的那张从语句里删掉。

-- SET @d = 'YYYYMMDD';
-- SET @sql = CONCAT(
--   'RENAME TABLE ',
--   'md_users TO _archived_md_users_', @d, ', ',
--   'md_departments TO _archived_md_departments_', @d, ', ',
--   'md_folders TO _archived_md_folders_', @d);
-- PREPARE s FROM @sql; EXECUTE s; DEALLOCATE PREPARE s;
--
-- -- 确认结果
-- SELECT TABLE_NAME FROM information_schema.TABLES
-- WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME LIKE '\_archived\_md\_%';

-- ============================================================
-- 第 3 节：回滚（改回原名；@d 用第 2 节同一个日期）
-- ============================================================

-- SET @d = 'YYYYMMDD';
-- SET @sql = CONCAT(
--   'RENAME TABLE ',
--   '_archived_md_users_', @d, ' TO md_users, ',
--   '_archived_md_departments_', @d, ' TO md_departments, ',
--   '_archived_md_folders_', @d, ' TO md_folders');
-- PREPARE s FROM @sql; EXECUTE s; DEALLOCATE PREPARE s;

-- ============================================================
-- 第 4 节：删除归档表（不可逆！观察期结束、再次备份后才执行）
-- ============================================================

-- SET @d = 'YYYYMMDD';
-- SET @sql = CONCAT(
--   'DROP TABLE ',
--   '_archived_md_users_', @d, ', ',
--   '_archived_md_departments_', @d, ', ',
--   '_archived_md_folders_', @d);
-- PREPARE s FROM @sql; EXECUTE s; DEALLOCATE PREPARE s;
