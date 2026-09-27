-- ⚠️ 仅供本地开发 / 演示使用，不要导入生产库。
-- Dev/demo only. Never load this into production.
--
-- In production these tables belong to the MistLab Portal (mist-team-server)
-- and live in the same database as MistDocs' md_* tables. This file creates
-- the minimal columns MistDocs reads, plus one demo team and admin so a fresh
-- install can be opened without the Portal:
--
--   go run ./cmd/devtoken -c configs/config.yaml -user dev-admin
--
-- prints a token and a /auth/callback URL that signs you in.
SET NAMES utf8mb4;

CREATE TABLE IF NOT EXISTS users (
  id             VARCHAR(64)  PRIMARY KEY,
  email          VARCHAR(255) NOT NULL DEFAULT '',
  username       VARCHAR(100) NOT NULL DEFAULT '',
  display_name   VARCHAR(100) NOT NULL DEFAULT '',
  password_hash  VARCHAR(255) NOT NULL DEFAULT '',
  is_admin       TINYINT(1)   NOT NULL DEFAULT 0,
  email_verified TINYINT(1)   NOT NULL DEFAULT 1,
  created_at     DATETIME     DEFAULT CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS teams (
  id          VARCHAR(64)  PRIMARY KEY,
  name        VARCHAR(200) NOT NULL,
  description VARCHAR(500) NOT NULL DEFAULT '',
  created_at  DATETIME     DEFAULT CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- role: owner / admin / editor / viewer (Portal roles; owner = admin in MistDocs)
CREATE TABLE IF NOT EXISTS team_members (
  team_id    VARCHAR(64) NOT NULL,
  user_id    VARCHAR(64) NOT NULL,
  role       VARCHAR(32) NOT NULL,
  created_at DATETIME    DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (team_id, user_id),
  INDEX idx_user (user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Team snippets (mist-team-server). MistDocs only links to them.
CREATE TABLE IF NOT EXISTS fragments (
  id       VARCHAR(64)  PRIMARY KEY,
  team_id  VARCHAR(64)  NOT NULL,
  title    VARCHAR(200) NOT NULL,
  command  TEXT,
  category VARCHAR(100) NOT NULL DEFAULT '',
  status   VARCHAR(32)  NOT NULL DEFAULT 'published',
  deleted  TINYINT      NOT NULL DEFAULT 0,
  INDEX idx_team (team_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Demo team and accounts (no passwords: sign in with cmd/devtoken).
INSERT IGNORE INTO teams (id, name) VALUES ('dev-team', '演示团队');
INSERT IGNORE INTO users (id, email, username, display_name) VALUES
  ('dev-admin',  'admin@example.invalid',  'admin',  '管理员'),
  ('dev-editor', 'editor@example.invalid', 'editor', '编辑者'),
  ('dev-viewer', 'viewer@example.invalid', 'viewer', '查看者');
INSERT IGNORE INTO team_members (team_id, user_id, role) VALUES
  ('dev-team', 'dev-admin', 'admin'),
  ('dev-team', 'dev-editor', 'editor'),
  ('dev-team', 'dev-viewer', 'viewer');
