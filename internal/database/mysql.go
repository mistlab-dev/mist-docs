package database

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/c-wind/mist-docs/internal/config"
	_ "github.com/go-sql-driver/mysql"
)

var DB *sql.DB

func Init(cfg config.DatabaseConfig) error {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=true&loc=Local",
		cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.DBName,
	)

	var err error
	DB, err = sql.Open("mysql", dsn)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}

	DB.SetMaxOpenConns(cfg.MaxOpenConns)
	DB.SetMaxIdleConns(cfg.MaxIdleConns)
	DB.SetConnMaxLifetime(time.Hour)

	if err = DB.Ping(); err != nil {
		return fmt.Errorf("ping db: %w", err)
	}

	// Auto-migrate: ensure content_text column exists for full-text search
	var colExists int
	DB.QueryRow(`SELECT COUNT(*) FROM INFORMATION_SCHEMA.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='md_documents' AND COLUMN_NAME='content_text'`).Scan(&colExists)
	if colExists == 0 {
		DB.Exec(`ALTER TABLE md_documents ADD COLUMN content_text LONGTEXT DEFAULT NULL`)
		DB.Exec(`ALTER TABLE md_documents ADD FULLTEXT INDEX ft_content_text (content_text)`)
	}

	// Auto-migrate: md_templates table
	var tblExists int
	DB.QueryRow(`SELECT COUNT(*) FROM INFORMATION_SCHEMA.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='md_templates'`).Scan(&tblExists)
	if tblExists == 0 {
		DB.Exec(`CREATE TABLE md_templates (
			id VARCHAR(36) PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			type VARCHAR(20) NOT NULL DEFAULT 'doc',
			content LONGTEXT,
			user_id VARCHAR(36) NOT NULL,
			department_id VARCHAR(36) DEFAULT '',
			is_public TINYINT(1) DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			INDEX idx_user (user_id),
			INDEX idx_dept (department_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`)
	}

	// Auto-migrate: md_team_folders table (team-scoped folder tree)
	DB.QueryRow(`SELECT COUNT(*) FROM INFORMATION_SCHEMA.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='md_team_folders'`).Scan(&tblExists)
	if tblExists == 0 {
		DB.Exec(`CREATE TABLE md_team_folders (
			id VARCHAR(36) PRIMARY KEY,
			team_id VARCHAR(64) NOT NULL,
			parent_id VARCHAR(36) DEFAULT '',
			name VARCHAR(200) NOT NULL,
			sort_order INT DEFAULT 0,
			created_by VARCHAR(64) NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			INDEX idx_team (team_id),
			INDEX idx_parent (parent_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`)
	}

	// Auto-migrate: md_doc_fragments 关联表（文档 ↔ 团队片段）
	// 片段数据位于同一库的 fragments 表(mist-team-server),此处仅存关联与顺序
	DB.QueryRow(`SELECT COUNT(*) FROM INFORMATION_SCHEMA.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='md_doc_fragments'`).Scan(&tblExists)
	if tblExists == 0 {
		DB.Exec(`CREATE TABLE md_doc_fragments (
			id VARCHAR(36) PRIMARY KEY,
			team_id VARCHAR(64) NOT NULL,
			document_id VARCHAR(36) NOT NULL COLLATE utf8mb4_general_ci,
			fragment_id VARCHAR(64) NOT NULL COLLATE utf8mb4_unicode_ci,
			position INT NOT NULL DEFAULT 0,
			created_by VARCHAR(64) NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE KEY uk_doc_frag (document_id, fragment_id),
			INDEX idx_team (team_id),
			INDEX idx_doc (document_id),
			INDEX idx_frag (fragment_id),
			CONSTRAINT fk_docfrag_doc FOREIGN KEY (document_id) REFERENCES md_documents(id) ON DELETE CASCADE
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`)
	}

	// Auto-migrate: 对齐 md_doc_fragments 关联字段 collation，避免与 fragments/源表 JOIN 冲突
	// fragments.id 为 utf8mb4_unicode_ci，md_documents.id 为 utf8mb4_general_ci
	DB.Exec(`ALTER TABLE md_doc_fragments MODIFY fragment_id VARCHAR(64) NOT NULL COLLATE utf8mb4_unicode_ci`)
	DB.Exec(`ALTER TABLE md_doc_fragments MODIFY document_id VARCHAR(36) NOT NULL COLLATE utf8mb4_general_ci`)

	// Older installs created these tables before team scope and updated_at.
	// Adding a missing column keeps existing rows.
	ensureColumn("md_webhooks", "team_id", "team_id VARCHAR(64) DEFAULT ''")
	ensureColumn("md_webhooks", "updated_at", "updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP")
	ensureColumn("md_templates", "team_id", "team_id VARCHAR(64) DEFAULT ''")

	return migrateDeadlines()
}

func ensureColumn(table, column, ddl string) {
	var n int
	if err := DB.QueryRow(
		`SELECT COUNT(*) FROM INFORMATION_SCHEMA.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME=? AND COLUMN_NAME=?`,
		table, column,
	).Scan(&n); err != nil || n > 0 {
		return
	}
	if _, err := DB.Exec(`ALTER TABLE ` + table + ` ADD COLUMN ` + ddl); err != nil {
		fmt.Printf("migrate %s.%s: %v\n", table, column, err)
	}
}

// migrateDeadlines creates the交期看板 (deadline board) tables.
//
// Design note: deadlines are stored as DB rows with a due_date index, NOT as
// spreadsheet documents. A sheet is an opaque encrypted blob that the server
// cannot query; "which orders are due in 3 days" would otherwise require
// decrypting and parsing every sheet. Keeping due_date as an indexed column
// makes the reminder scan a plain date range query.
func migrateDeadlines() error {
	var tblExists int

	// 1. md_deadlines — the ledger. Single source of truth for the board.
	DB.QueryRow(`SELECT COUNT(*) FROM INFORMATION_SCHEMA.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='md_deadlines'`).Scan(&tblExists)
	if tblExists == 0 {
		if _, err := DB.Exec(`CREATE TABLE md_deadlines (
			id VARCHAR(36) PRIMARY KEY,
			team_id VARCHAR(64) NOT NULL,
			order_no VARCHAR(64) NOT NULL,
			title VARCHAR(255) NOT NULL,
			customer VARCHAR(128) DEFAULT '',
			quantity INT DEFAULT 0,
			start_date DATE DEFAULT NULL,
			due_date DATE NOT NULL,
			status VARCHAR(16) NOT NULL DEFAULT 'pending',
			progress TINYINT DEFAULT 0,
			priority VARCHAR(16) NOT NULL DEFAULT 'normal',
			owner_id VARCHAR(64) DEFAULT '',
			remark TEXT,
			created_by VARCHAR(64) NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_by VARCHAR(64) DEFAULT '',
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			deleted_at DATETIME DEFAULT NULL,
			INDEX idx_due (team_id, due_date, status),
			INDEX idx_owner (team_id, owner_id),
			INDEX idx_status (team_id, status),
			INDEX idx_order (team_id, order_no)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`); err != nil {
			return fmt.Errorf("create md_deadlines: %w", err)
		}
	}

	// 2. md_reminder_rules — configurable T-N reminder rules.
	DB.QueryRow(`SELECT COUNT(*) FROM INFORMATION_SCHEMA.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='md_reminder_rules'`).Scan(&tblExists)
	if tblExists == 0 {
		if _, err := DB.Exec(`CREATE TABLE md_reminder_rules (
			id VARCHAR(36) PRIMARY KEY,
			team_id VARCHAR(64) NOT NULL,
			name VARCHAR(64) NOT NULL,
			offset_days INT NOT NULL COMMENT 'days before due date; 0 = due today; negative = overdue',
			channel VARCHAR(32) NOT NULL DEFAULT 'inapp' COMMENT 'inapp|webhook',
			target VARCHAR(32) NOT NULL DEFAULT 'owner' COMMENT 'owner|creator',
			enabled TINYINT(1) NOT NULL DEFAULT 1,
			created_by VARCHAR(64) NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			INDEX idx_team (team_id, enabled)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`); err != nil {
			return fmt.Errorf("create md_reminder_rules: %w", err)
		}
	}

	// 3. md_reminder_log — the idempotency guard.
	// The scheduler runs repeatedly; the UNIQUE key is what prevents the same
	// rule+deadline from notifying more than once. Without it users get
	// bombarded and switch notifications off, which kills the feature.
	DB.QueryRow(`SELECT COUNT(*) FROM INFORMATION_SCHEMA.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='md_reminder_log'`).Scan(&tblExists)
	if tblExists == 0 {
		if _, err := DB.Exec(`CREATE TABLE md_reminder_log (
			id VARCHAR(36) PRIMARY KEY,
			rule_id VARCHAR(36) NOT NULL,
			deadline_id VARCHAR(36) NOT NULL,
			team_id VARCHAR(64) NOT NULL,
			target_user_id VARCHAR(64) DEFAULT '',
			channel VARCHAR(32) NOT NULL DEFAULT 'inapp',
			result VARCHAR(16) NOT NULL DEFAULT 'ok',
			detail VARCHAR(255) DEFAULT '',
			sent_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE KEY uk_once (rule_id, deadline_id),
			INDEX idx_deadline (deadline_id),
			INDEX idx_team_time (team_id, sent_at)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`); err != nil {
			return fmt.Errorf("create md_reminder_log: %w", err)
		}
	}

	// 4. md_deadline_events — change history.
	// Historical data cannot be reconstructed later, so this is written from
	// day one. `reason` is optional but is the most valuable column: it is the
	// fuel for any future "why was this order pushed back" explanation.
	DB.QueryRow(`SELECT COUNT(*) FROM INFORMATION_SCHEMA.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='md_deadline_events'`).Scan(&tblExists)
	if tblExists == 0 {
		if _, err := DB.Exec(`CREATE TABLE md_deadline_events (
			id VARCHAR(36) PRIMARY KEY,
			team_id VARCHAR(64) NOT NULL,
			deadline_id VARCHAR(36) NOT NULL,
			event_type VARCHAR(32) NOT NULL COMMENT 'created|updated|status_changed|date_changed|deleted',
			field VARCHAR(32) DEFAULT '',
			old_value TEXT,
			new_value TEXT,
			reason TEXT,
			actor_id VARCHAR(64) DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			INDEX idx_deadline (deadline_id, created_at),
			INDEX idx_team (team_id, created_at)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`); err != nil {
			return fmt.Errorf("create md_deadline_events: %w", err)
		}
	}

	return nil
}

func Close() {
	if DB != nil {
		DB.Close()
	}
}
