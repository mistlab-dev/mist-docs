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
	return nil
}

func Close() {
	if DB != nil {
		DB.Close()
	}
}
