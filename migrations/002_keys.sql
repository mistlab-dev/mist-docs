-- MistDocs 密钥管理表（AES-256-GCM 加密）
-- Master Key 加密 DEK，DEK 加密文档数据

CREATE TABLE IF NOT EXISTS md_keys (
    id VARCHAR(36) PRIMARY KEY,
    type VARCHAR(10) NOT NULL COMMENT 'master/dek/kek',
    status VARCHAR(10) NOT NULL DEFAULT 'active' COMMENT 'active/inactive/rotated',
    encrypted TEXT NOT NULL COMMENT 'base64 encoded encrypted key material',
    algorithm VARCHAR(20) NOT NULL DEFAULT 'AES-256-GCM',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    rotated_from VARCHAR(36) DEFAULT NULL COMMENT 'previous key ID during rotation',
    created_by VARCHAR(100) DEFAULT 'system',
    INDEX idx_type_status (type, status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
