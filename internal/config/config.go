package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server    ServerConfig    `yaml:"server"`
	Database  DatabaseConfig  `yaml:"database"`
	Storage   StorageConfig   `yaml:"storage"`
	JWT       JWTConfig       `yaml:"jwt"`
	WebSocket WebSocketConfig `yaml:"websocket"`
	Audit     AuditConfig     `yaml:"audit"`
	Portal    PortalConfig    `yaml:"portal"`
	Log       LogConfig       `yaml:"log"`
}

type ServerConfig struct {
	Host    string `yaml:"host"`
	Port    int    `yaml:"port"`
	BaseURL string `yaml:"base_url"`
}

type DatabaseConfig struct {
	Host         string `yaml:"host"`
	Port         int    `yaml:"port"`
	User         string `yaml:"user"`
	Password     string `yaml:"password"`
	DBName       string `yaml:"dbname"`
	MaxOpenConns int    `yaml:"max_open_conns"`
	MaxIdleConns int    `yaml:"max_idle_conns"`
}

type StorageConfig struct {
	Root        string `yaml:"root"`
	MaxFileSize int64  `yaml:"max_file_size"`
	VersionKeep int    `yaml:"version_keep"`
}

type JWTConfig struct {
	Secret      string `yaml:"secret"`
	ExpireHours int    `yaml:"expire_hours"`
	Issuer      string `yaml:"issuer"`
}

type WebSocketConfig struct {
	ReadBufferSize  int `yaml:"read_buffer_size"`
	WriteBufferSize int `yaml:"write_buffer_size"`
	PingInterval    int `yaml:"ping_interval"`
	MaxMessageSize  int64 `yaml:"max_message_size"`
}

type AuditConfig struct {
	RetainDays int `yaml:"retain_days"`
}

type PortalConfig struct {
	URL string `yaml:"url"` // Portal base URL, e.g. https://mistlab.dev
}

type LogConfig struct {
	Level string `yaml:"level"`
	File  string `yaml:"file"`
}

var C Config

func Load(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := yaml.Unmarshal(data, &C); err != nil {
		return err
	}
	// 环境变量覆盖
	envOverride()
	return nil
}

func envOverride() {
	if v := os.Getenv("DB_HOST"); v != "" {
		C.Database.Host = v
	}
	if v := os.Getenv("DB_PORT"); v != "" {
		C.Database.Port = atoi(v, C.Database.Port)
	}
	if v := os.Getenv("DB_USER"); v != "" {
		C.Database.User = v
	}
	if v := os.Getenv("DB_PASS"); v != "" {
		C.Database.Password = v
	}
	if v := os.Getenv("DB_NAME"); v != "" {
		C.Database.DBName = v
	}
	if v := os.Getenv("SERVER_PORT"); v != "" {
		C.Server.Port = atoi(v, C.Server.Port)
	}
	if v := os.Getenv("JWT_SECRET"); v != "" {
		C.JWT.Secret = v
	}
	if v := os.Getenv("DATA_DIR"); v != "" {
		C.Storage.Root = v + "/files"
	}
	if v := os.Getenv("LOG_LEVEL"); v != "" {
		C.Log.Level = v
	}
}

func atoi(s string, fallback int) int {
	n := 0
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			return fallback
		}
		n = n*10 + int(ch-'0')
	}
	return n
}
