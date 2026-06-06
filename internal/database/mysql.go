package database

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/c-wind/mist-docs/internal/config"
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

	return nil
}

func Close() {
	if DB != nil {
		DB.Close()
	}
}
