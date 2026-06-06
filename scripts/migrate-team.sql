-- ============================================================
-- MistDocs 统一多租户 Migration
-- Phase 1: Schema 变更 + 数据迁移
-- 
-- 前置条件：主站 users/teams/team_members 表已存在
-- 执行顺序：先 schema，后数据迁移
-- ============================================================

-- ============================================================
-- Part 1: Schema 变更（可重复执行，IF NOT EXISTS）
-- ============================================================

-- 1.1 新建团队文件夹表（替代 md_departments）
CREATE TABLE IF NOT EXISTS md_team_folders (
  id          VARCHAR(36) PRIMARY KEY,
  team_id     VARCHAR(64) NOT NULL,
  parent_id   VARCHAR(36) DEFAULT '',
  name        VARCHAR(200) NOT NULL,
  sort_order  INT DEFAULT 0,
  created_by  VARCHAR(64) NOT NULL,
  created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
  updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  INDEX idx_team (team_id),
  INDEX idx_parent (parent_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 1.2 md_documents 加 team_id
SET @col_exists = 0;
SELECT COUNT(*) INTO @col_exists 
FROM INFORMATION_SCHEMA.COLUMNS 
WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='md_documents' AND COLUMN_NAME='team_id';

SET @sql = IF(@col_exists = 0, 
  'ALTER TABLE md_documents ADD COLUMN team_id VARCHAR(64) DEFAULT '''' AFTER id, ADD INDEX idx_md_docs_team (team_id)',
  'SELECT ''md_documents.team_id already exists''');
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

-- 1.3 md_folders 加 team_id
SET @col_exists = 0;
SELECT COUNT(*) INTO @col_exists 
FROM INFORMATION_SCHEMA.COLUMNS 
WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='md_folders' AND COLUMN_NAME='team_id';

SET @sql = IF(@col_exists = 0, 
  'ALTER TABLE md_folders ADD COLUMN team_id VARCHAR(64) DEFAULT '''' AFTER id, ADD INDEX idx_md_folders_team (team_id)',
  'SELECT ''md_folders.team_id already exists''');
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

-- 1.4 md_audits 加 team_id
SET @col_exists = 0;
SELECT COUNT(*) INTO @col_exists 
FROM INFORMATION_SCHEMA.COLUMNS 
WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='md_audits' AND COLUMN_NAME='team_id';

SET @sql = IF(@col_exists = 0, 
  'ALTER TABLE md_audits ADD COLUMN team_id VARCHAR(64) DEFAULT ''''',
  'SELECT ''md_audits.team_id already exists''');
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

-- 1.5 md_shares 加 team_id
SET @col_exists = 0;
SELECT COUNT(*) INTO @col_exists 
FROM INFORMATION_SCHEMA.COLUMNS 
WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='md_shares' AND COLUMN_NAME='team_id';

SET @sql = IF(@col_exists = 0, 
  'ALTER TABLE md_shares ADD COLUMN team_id VARCHAR(64) DEFAULT ''''',
  'SELECT ''md_shares.team_id already exists''');
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

-- 1.6 md_comments 加 team_id
SET @col_exists = 0;
SELECT COUNT(*) INTO @col_exists 
FROM INFORMATION_SCHEMA.COLUMNS 
WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='md_comments' AND COLUMN_NAME='team_id';

SET @sql = IF(@col_exists = 0, 
  'ALTER TABLE md_comments ADD COLUMN team_id VARCHAR(64) DEFAULT ''''',
  'SELECT ''md_comments.team_id already exists''');
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

-- 1.7 md_notifications 加 team_id
SET @col_exists = 0;
SELECT COUNT(*) INTO @col_exists 
FROM INFORMATION_SCHEMA.COLUMNS 
WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='md_notifications' AND COLUMN_NAME='team_id';

SET @sql = IF(@col_exists = 0, 
  'ALTER TABLE md_notifications ADD COLUMN team_id VARCHAR(64) DEFAULT ''''',
  'SELECT ''md_notifications.team_id already exists''');
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

-- 1.8 md_tags 加 team_id
SET @col_exists = 0;
SELECT COUNT(*) INTO @col_exists 
FROM INFORMATION_SCHEMA.COLUMNS 
WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='md_tags' AND COLUMN_NAME='team_id';

SET @sql = IF(@col_exists = 0, 
  'ALTER TABLE md_tags ADD COLUMN team_id VARCHAR(64) DEFAULT ''''',
  'SELECT ''md_tags.team_id already exists''');
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

-- 1.9 md_templates 加 team_id
SET @col_exists = 0;
SELECT COUNT(*) INTO @col_exists 
FROM INFORMATION_SCHEMA.COLUMNS 
WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='md_templates' AND COLUMN_NAME='team_id';

SET @sql = IF(@col_exists = 0, 
  'ALTER TABLE md_templates ADD COLUMN team_id VARCHAR(64) DEFAULT '''' AFTER id',
  'SELECT ''md_templates.team_id already exists''');
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

-- ============================================================
-- Part 2: 数据迁移
-- ============================================================

-- 2.1 建立 md_users → users ID 映射表（临时）
-- tgy (md: 4d2d...) → u_32e4fd26-2b7 (主站: 夕阳武士, google login)
-- admin (md: u_admin) → u_adm_8f9078de-9f0 (主站: Admin)
-- 其他 md_users 没有主站对应账号，暂时不动

CREATE TEMPORARY TABLE md_user_mapping (
  md_user_id VARCHAR(64) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci PRIMARY KEY,
  main_user_id VARCHAR(64) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL,
  md_username VARCHAR(50) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci
);

INSERT INTO md_user_mapping VALUES
  ('u_admin', 'u_adm_8f9078de-9f0', 'admin'),
  ('4d2d23ab-f29c-4e29-b272-e1d57e009671', 'u_32e4fd26-2b7', 'tgy');

-- 2.2 迁移 md_folders → md_team_folders
-- 所有现有文件夹都归入 team_31ece9fd-471（admin 的团队）
-- 现有 folder 结构:
--   项目资料 (root)
--     ├─ 需求
--     └─ 2026计划

INSERT IGNORE INTO md_team_folders (id, team_id, parent_id, name, sort_order, created_by, created_at, updated_at)
SELECT 
  f.id,
  'team_31ece9fd-471' as team_id,
  COALESCE(f.parent_id, '') as parent_id,
  f.name,
  0 as sort_order,
  COALESCE(m.main_user_id, 'u_adm_8f9078de-9f0') as created_by,
  f.created_at,
  f.updated_at
FROM md_folders f
LEFT JOIN md_user_mapping m ON f.created_by = m.md_user_id;

-- 2.3 更新 md_folders.team_id
UPDATE md_folders f SET f.team_id = 'team_31ece9fd-471' WHERE f.team_id = '';

-- 2.4 更新 md_documents.team_id
UPDATE md_documents d SET d.team_id = 'team_31ece9fd-471' WHERE d.team_id = '';

-- 2.5 更新 md_documents.created_by（映射到主站 user_id）
UPDATE md_documents d
INNER JOIN md_user_mapping m ON d.created_by = m.md_user_id
SET d.created_by = m.main_user_id;

-- 2.6 更新 md_documents.updated_by
UPDATE md_documents d
INNER JOIN md_user_mapping m ON d.updated_by = m.md_user_id
SET d.updated_by = m.main_user_id;

-- 2.7 更新 md_folders.created_by
UPDATE md_folders f
INNER JOIN md_user_mapping m ON f.created_by = m.md_user_id
SET f.created_by = m.main_user_id;

-- 2.8 更新 md_permissions（user target 映射）
UPDATE md_permissions p
INNER JOIN md_user_mapping m ON p.target_id = m.md_user_id AND p.target_type = 'user'
SET p.target_id = m.main_user_id;

-- 2.9 更新 md_permissions created_by
UPDATE md_permissions p
INNER JOIN md_user_mapping m ON p.created_by = m.md_user_id
SET p.created_by = m.main_user_id;

-- 2.10 更新 md_audits user 映射
UPDATE md_audits a
INNER JOIN md_user_mapping m ON a.user_id = m.md_user_id
SET a.user_id = m.main_user_id;

-- 2.11 更新 md_audits team_id（全部归入默认团队）
UPDATE md_audits SET team_id = 'team_31ece9fd-471' WHERE team_id = '';

-- 2.12 更新 md_shares created_by
UPDATE md_shares s
INNER JOIN md_user_mapping m ON s.created_by = m.md_user_id
SET s.created_by = m.main_user_id;
UPDATE md_shares SET team_id = 'team_31ece9fd-471' WHERE team_id = '';

-- 2.13 更新 md_comments
UPDATE md_comments c
INNER JOIN md_user_mapping m ON c.user_id = m.md_user_id
SET c.user_id = m.main_user_id;
UPDATE md_comments SET team_id = 'team_31ece9fd-471' WHERE team_id = '';

-- 2.14 更新 md_tags
UPDATE md_tags SET team_id = 'team_31ece9fd-471' WHERE team_id = '';

-- 2.15 更新 md_templates
UPDATE md_templates t
INNER JOIN md_user_mapping m ON t.user_id = m.md_user_id
SET t.user_id = m.main_user_id;
UPDATE md_templates SET team_id = 'team_31ece9fd-471' WHERE team_id = '';

-- 清理临时表
DROP TEMPORARY TABLE md_user_mapping;

-- ============================================================
-- Part 3: 数据验证（手动执行确认）
-- ============================================================

-- 验证 team_id 已填充
-- SELECT COUNT(*) as docs_without_team FROM md_documents WHERE team_id = '';
-- SELECT COUNT(*) as folders_without_team FROM md_folders WHERE team_id = '';
-- SELECT COUNT(*) as audits_without_team FROM md_audits WHERE team_id = '';

-- 验证 md_team_folders 数据
-- SELECT * FROM md_team_folders;

-- 验证 user_id 映射
-- SELECT id, title, created_by, team_id FROM md_documents;

-- ============================================================
-- Part 4: 清理（确认数据无误后手动执行）
-- ============================================================

-- DROP TABLE IF EXISTS md_departments;
-- DROP TABLE IF EXISTS md_users;
-- 注意：删除前确保所有外键引用已更新
