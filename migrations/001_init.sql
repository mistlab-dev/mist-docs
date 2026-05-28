-- MistDocs 初始化数据库

CREATE TABLE IF NOT EXISTS departments (
    id VARCHAR(36) PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    parent_id VARCHAR(36) DEFAULT NULL,
    sort_order INT DEFAULT 0,
    status TINYINT DEFAULT 1,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_parent (parent_id),
    CONSTRAINT fk_dept_parent FOREIGN KEY (parent_id) REFERENCES departments(id) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS users (
    id VARCHAR(36) PRIMARY KEY,
    username VARCHAR(50) NOT NULL UNIQUE,
    password VARCHAR(255) NOT NULL,
    name VARCHAR(100) NOT NULL,
    email VARCHAR(100) DEFAULT NULL,
    phone VARCHAR(20) DEFAULT NULL,
    department_id VARCHAR(36) DEFAULT NULL,
    role VARCHAR(20) NOT NULL DEFAULT 'member',
    status TINYINT DEFAULT 1,
    last_login_at DATETIME DEFAULT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_dept (department_id),
    INDEX idx_username (username),
    CONSTRAINT fk_user_dept FOREIGN KEY (department_id) REFERENCES departments(id) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS doc_folders (
    id VARCHAR(36) PRIMARY KEY,
    name VARCHAR(200) NOT NULL,
    parent_id VARCHAR(36) DEFAULT NULL,
    department_id VARCHAR(36) NOT NULL,
    created_by VARCHAR(36) DEFAULT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_parent (parent_id),
    INDEX idx_dept (department_id),
    CONSTRAINT fk_folder_parent FOREIGN KEY (parent_id) REFERENCES doc_folders(id) ON DELETE CASCADE,
    CONSTRAINT fk_folder_dept FOREIGN KEY (department_id) REFERENCES departments(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS documents (
    id VARCHAR(36) PRIMARY KEY,
    folder_id VARCHAR(36) DEFAULT NULL,
    department_id VARCHAR(36) NOT NULL,
    title VARCHAR(200) NOT NULL,
    type VARCHAR(20) NOT NULL DEFAULT 'doc',
    file_path VARCHAR(500) DEFAULT NULL,
    file_size BIGINT DEFAULT 0,
    version INT DEFAULT 1,
    status TINYINT DEFAULT 1,
    created_by VARCHAR(36) DEFAULT NULL,
    updated_by VARCHAR(36) DEFAULT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_folder (folder_id),
    INDEX idx_dept (department_id),
    INDEX idx_status (status),
    INDEX idx_type (type),
    CONSTRAINT fk_doc_folder FOREIGN KEY (folder_id) REFERENCES doc_folders(id) ON DELETE SET NULL,
    CONSTRAINT fk_doc_dept FOREIGN KEY (department_id) REFERENCES departments(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS doc_versions (
    id VARCHAR(36) PRIMARY KEY,
    document_id VARCHAR(36) NOT NULL,
    version INT NOT NULL,
    file_path VARCHAR(500) NOT NULL,
    file_size BIGINT DEFAULT 0,
    created_by VARCHAR(36) DEFAULT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_doc (document_id),
    UNIQUE KEY uk_doc_version (document_id, version),
    CONSTRAINT fk_version_doc FOREIGN KEY (document_id) REFERENCES documents(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS doc_permissions (
    id VARCHAR(36) PRIMARY KEY,
    resource_type VARCHAR(10) NOT NULL,
    resource_id VARCHAR(36) NOT NULL,
    target_type VARCHAR(10) NOT NULL,
    target_id VARCHAR(36) NOT NULL,
    permission VARCHAR(10) NOT NULL DEFAULT 'read',
    inherit TINYINT DEFAULT 1,
    created_by VARCHAR(36) DEFAULT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_resource (resource_type, resource_id),
    INDEX idx_target (target_type, target_id),
    UNIQUE KEY uk_perm (resource_type, resource_id, target_type, target_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS doc_audits (
    id VARCHAR(36) PRIMARY KEY,
    user_id VARCHAR(36) NOT NULL,
    user_name VARCHAR(100) DEFAULT NULL,
    department_id VARCHAR(36) DEFAULT NULL,
    action VARCHAR(30) NOT NULL,
    resource_type VARCHAR(10) DEFAULT NULL,
    resource_id VARCHAR(36) DEFAULT NULL,
    resource_name VARCHAR(200) DEFAULT NULL,
    detail TEXT DEFAULT NULL,
    ip VARCHAR(50) DEFAULT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_user (user_id),
    INDEX idx_dept (department_id),
    INDEX idx_action (action),
    INDEX idx_resource (resource_type, resource_id),
    INDEX idx_time (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 默认超级管理员（密码: Admin@2026）
INSERT IGNORE INTO users (id, username, password, name, role) VALUES
('u_admin', 'admin', '$2a$10$placeholder', '超级管理员', 'super_admin');
