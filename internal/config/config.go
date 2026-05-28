package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server     ServerConfig     `yaml:"server"`
	Database   DatabaseConfig   `yaml:"database"`
	Storage    StorageConfig    `yaml:"storage"`
	JWT        JWTConfig        `yaml:"jwt"`
	WebSocket  WebSocketConfig  `yaml:"websocket"`
	Audit      AuditConfig      `yaml:"audit"`
	MistTerm   MistTermConfig   `yaml:"mistterm"`
	Log        LogConfig        `yaml:"log"`
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

type MistTermConfig struct {
	APIURL string `yaml:"api_url"`
	APIKey string `yaml:"api_key"`
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
	return yaml.Unmarshal(data, &C)
}
