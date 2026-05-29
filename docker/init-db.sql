-- table: md_departments
CREATE TABLE IF NOT EXISTS `md_departments` (
  `id` varchar(36) NOT NULL,
  `name` varchar(100) NOT NULL,
  `parent_id` varchar(36) DEFAULT NULL,
  `sort_order` int(11) DEFAULT 0,
  `status` tinyint(4) DEFAULT 1,
  `created_at` datetime DEFAULT current_timestamp(),
  `updated_at` datetime DEFAULT current_timestamp() ON UPDATE current_timestamp(),
  PRIMARY KEY (`id`),
  KEY `idx_parent` (`parent_id`),
  CONSTRAINT `fk_md_dept_parent` FOREIGN KEY (`parent_id`) REFERENCES `md_departments` (`id`) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;
-- table: md_users
CREATE TABLE IF NOT EXISTS `md_users` (
  `id` varchar(36) NOT NULL,
  `username` varchar(50) NOT NULL,
  `password` varchar(255) NOT NULL,
  `name` varchar(100) NOT NULL,
  `email` varchar(100) DEFAULT NULL,
  `phone` varchar(20) DEFAULT NULL,
  `department_id` varchar(36) DEFAULT NULL,
  `role` varchar(20) NOT NULL DEFAULT 'member',
  `status` tinyint(4) DEFAULT 1,
  `last_login_at` datetime DEFAULT NULL,
  `created_at` datetime DEFAULT current_timestamp(),
  `updated_at` datetime DEFAULT current_timestamp() ON UPDATE current_timestamp(),
  PRIMARY KEY (`id`),
  UNIQUE KEY `username` (`username`),
  KEY `idx_dept` (`department_id`),
  KEY `idx_username` (`username`),
  CONSTRAINT `fk_md_user_dept` FOREIGN KEY (`department_id`) REFERENCES `md_departments` (`id`) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;
-- table: md_folders
CREATE TABLE IF NOT EXISTS `md_folders` (
  `id` varchar(36) NOT NULL,
  `name` varchar(200) NOT NULL,
  `parent_id` varchar(36) DEFAULT NULL,
  `department_id` varchar(36) NOT NULL,
  `created_by` varchar(36) DEFAULT NULL,
  `created_at` datetime DEFAULT current_timestamp(),
  `updated_at` datetime DEFAULT current_timestamp() ON UPDATE current_timestamp(),
  PRIMARY KEY (`id`),
  KEY `idx_parent` (`parent_id`),
  KEY `idx_dept` (`department_id`),
  CONSTRAINT `fk_md_folder_dept` FOREIGN KEY (`department_id`) REFERENCES `md_departments` (`id`),
  CONSTRAINT `fk_md_folder_parent` FOREIGN KEY (`parent_id`) REFERENCES `md_folders` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;
-- table: md_documents
CREATE TABLE IF NOT EXISTS `md_documents` (
  `id` varchar(36) NOT NULL,
  `folder_id` varchar(36) DEFAULT NULL,
  `department_id` varchar(36) NOT NULL,
  `title` varchar(200) NOT NULL,
  `type` varchar(20) NOT NULL DEFAULT 'doc',
  `file_path` varchar(500) DEFAULT NULL,
  `file_size` bigint(20) DEFAULT 0,
  `version` int(11) DEFAULT 1,
  `locked_by` varchar(36) DEFAULT '',
  `locked_at` datetime DEFAULT NULL,
  `status` tinyint(4) DEFAULT 1,
  `created_by` varchar(36) DEFAULT NULL,
  `updated_by` varchar(36) DEFAULT NULL,
  `created_at` datetime DEFAULT current_timestamp(),
  `updated_at` datetime DEFAULT current_timestamp() ON UPDATE current_timestamp(),
  PRIMARY KEY (`id`),
  KEY `idx_folder` (`folder_id`),
  KEY `idx_dept` (`department_id`),
  KEY `idx_status` (`status`),
  KEY `idx_type` (`type`),
  CONSTRAINT `fk_md_doc_dept` FOREIGN KEY (`department_id`) REFERENCES `md_departments` (`id`),
  CONSTRAINT `fk_md_doc_folder` FOREIGN KEY (`folder_id`) REFERENCES `md_folders` (`id`) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;
-- table: md_versions
CREATE TABLE IF NOT EXISTS `md_versions` (
  `id` varchar(36) NOT NULL,
  `document_id` varchar(36) NOT NULL,
  `version` int(11) NOT NULL,
  `file_path` varchar(500) NOT NULL,
  `file_size` bigint(20) DEFAULT 0,
  `created_by` varchar(36) DEFAULT NULL,
  `created_at` datetime DEFAULT current_timestamp(),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_doc_version` (`document_id`,`version`),
  KEY `idx_doc` (`document_id`),
  CONSTRAINT `fk_md_version_doc` FOREIGN KEY (`document_id`) REFERENCES `md_documents` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;
-- table: md_permissions
CREATE TABLE IF NOT EXISTS `md_permissions` (
  `id` varchar(36) NOT NULL,
  `resource_type` varchar(10) NOT NULL,
  `resource_id` varchar(36) NOT NULL,
  `target_type` varchar(10) NOT NULL,
  `target_id` varchar(36) NOT NULL,
  `permission` varchar(10) NOT NULL DEFAULT 'read',
  `inherit` tinyint(4) DEFAULT 1,
  `created_by` varchar(36) DEFAULT NULL,
  `created_at` datetime DEFAULT current_timestamp(),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_perm` (`resource_type`,`resource_id`,`target_type`,`target_id`),
  KEY `idx_resource` (`resource_type`,`resource_id`),
  KEY `idx_target` (`target_type`,`target_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;
-- table: md_audits
CREATE TABLE IF NOT EXISTS `md_audits` (
  `id` varchar(36) NOT NULL,
  `user_id` varchar(36) NOT NULL,
  `user_name` varchar(100) DEFAULT NULL,
  `department_id` varchar(36) DEFAULT NULL,
  `action` varchar(30) NOT NULL,
  `resource_type` varchar(10) DEFAULT NULL,
  `resource_id` varchar(36) DEFAULT NULL,
  `resource_name` varchar(200) DEFAULT NULL,
  `detail` text DEFAULT NULL,
  `ip` varchar(50) DEFAULT NULL,
  `created_at` datetime DEFAULT current_timestamp(),
  PRIMARY KEY (`id`),
  KEY `idx_user` (`user_id`),
  KEY `idx_dept` (`department_id`),
  KEY `idx_action` (`action`),
  KEY `idx_resource` (`resource_type`,`resource_id`),
  KEY `idx_time` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;
-- table: md_keys
CREATE TABLE IF NOT EXISTS `md_keys` (
  `id` varchar(36) NOT NULL,
  `type` varchar(10) NOT NULL COMMENT 'master/dek/kek',
  `status` varchar(10) NOT NULL DEFAULT 'active' COMMENT 'active/inactive/rotated',
  `encrypted` text NOT NULL COMMENT 'base64 encoded encrypted key material',
  `algorithm` varchar(20) NOT NULL DEFAULT 'AES-256-GCM',
  `created_at` datetime DEFAULT current_timestamp(),
  `rotated_from` varchar(36) DEFAULT NULL COMMENT 'previous key ID during rotation',
  `created_by` varchar(100) DEFAULT 'system',
  PRIMARY KEY (`id`),
  KEY `idx_type_status` (`type`,`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;
-- table: md_shares
CREATE TABLE IF NOT EXISTS `md_shares` (
  `id` varchar(36) NOT NULL,
  `document_id` varchar(36) NOT NULL,
  `token` varchar(64) NOT NULL,
  `password` varchar(100) DEFAULT '',
  `expires_at` datetime DEFAULT NULL,
  `created_by` varchar(36) NOT NULL,
  `status` tinyint(1) NOT NULL DEFAULT 1,
  `access_count` int(11) NOT NULL DEFAULT 0,
  `created_at` datetime NOT NULL DEFAULT current_timestamp(),
  PRIMARY KEY (`id`),
  UNIQUE KEY `token` (`token`),
  KEY `idx_share_doc` (`document_id`),
  KEY `idx_share_token` (`token`),
  KEY `idx_share_created_by` (`created_by`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;
-- table: md_comments
CREATE TABLE IF NOT EXISTS `md_comments` (
  `id` varchar(36) NOT NULL,
  `document_id` varchar(36) NOT NULL,
  `content` text NOT NULL,
  `parent_id` varchar(36) DEFAULT NULL,
  `user_id` varchar(36) NOT NULL,
  `user_name` varchar(100) NOT NULL,
  `created_at` datetime NOT NULL DEFAULT current_timestamp(),
  `updated_at` datetime NOT NULL DEFAULT current_timestamp() ON UPDATE current_timestamp(),
  PRIMARY KEY (`id`),
  KEY `idx_comment_doc` (`document_id`),
  KEY `idx_comment_parent` (`parent_id`),
  KEY `idx_comment_user` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;
-- table: md_notifications
CREATE TABLE IF NOT EXISTS `md_notifications` (
  `id` varchar(36) NOT NULL,
  `user_id` varchar(36) NOT NULL,
  `type` varchar(32) NOT NULL,
  `title` varchar(255) NOT NULL,
  `document_id` varchar(36) DEFAULT NULL,
  `related_id` varchar(36) DEFAULT NULL,
  `is_read` tinyint(1) NOT NULL DEFAULT 0,
  `created_at` datetime NOT NULL DEFAULT current_timestamp(),
  PRIMARY KEY (`id`),
  KEY `idx_notif_user` (`user_id`),
  KEY `idx_notif_read` (`user_id`,`is_read`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;
-- table: md_tags
CREATE TABLE IF NOT EXISTS `md_tags` (
  `id` varchar(36) NOT NULL,
  `name` varchar(50) NOT NULL,
  `color` varchar(7) DEFAULT '#409eff',
  `user_id` varchar(36) NOT NULL,
  `created_at` datetime DEFAULT current_timestamp(),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_user_tag` (`user_id`,`name`),
  KEY `idx_user` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;
-- table: md_doc_tags
CREATE TABLE IF NOT EXISTS `md_doc_tags` (
  `id` varchar(36) NOT NULL,
  `document_id` varchar(36) NOT NULL,
  `tag_id` varchar(36) NOT NULL,
  `created_at` datetime DEFAULT current_timestamp(),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_doc_tag` (`document_id`,`tag_id`),
  KEY `idx_doc` (`document_id`),
  KEY `idx_tag` (`tag_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;
-- table: md_favorites
CREATE TABLE IF NOT EXISTS `md_favorites` (
  `id` varchar(36) NOT NULL,
  `user_id` varchar(36) NOT NULL,
  `document_id` varchar(36) NOT NULL,
  `created_at` datetime DEFAULT current_timestamp(),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_user_doc` (`user_id`,`document_id`),
  KEY `idx_user_id` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;
-- table: md_webhooks
CREATE TABLE IF NOT EXISTS `md_webhooks` (
  `id` varchar(36) NOT NULL,
  `name` varchar(100) NOT NULL,
  `url` varchar(500) NOT NULL,
  `secret` varchar(200) DEFAULT '',
  `events` varchar(500) DEFAULT 'create,update,delete',
  `created_by` varchar(36) DEFAULT '',
  `created_at` datetime DEFAULT current_timestamp(),
  `enabled` tinyint(1) DEFAULT 1,
  PRIMARY KEY (`id`),
  KEY `idx_enabled` (`enabled`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;
-- table: md_webhook_logs
CREATE TABLE IF NOT EXISTS `md_webhook_logs` (
  `id` varchar(36) NOT NULL,
  `webhook_id` varchar(36) NOT NULL,
  `event` varchar(50) NOT NULL,
  `status` varchar(200) DEFAULT '',
  `created_at` datetime DEFAULT current_timestamp(),
  PRIMARY KEY (`id`),
  KEY `idx_webhook` (`webhook_id`),
  KEY `idx_time` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;

-- ─── 种子数据 ───

-- 默认部门
INSERT IGNORE INTO md_departments (id, name, sort_order) VALUES ('dept_default', '默认部门', 0);

-- 默认管理员（密码: Admin@2026，需首次启动后通过API设置）
INSERT IGNORE INTO md_users (id, username, password, name, department_id, role) 
VALUES ('u_admin', 'admin', '$2a$10$placeholder_need_reset', '超级管理员', 'dept_default', 'super_admin');
