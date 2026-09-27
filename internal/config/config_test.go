package config

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func resetC(t *testing.T) {
	t.Helper()
	old := C
	C = Config{}
	t.Cleanup(func() { C = old })
}

func clearEnv(t *testing.T) {
	for _, k := range []string{"DB_HOST", "DB_PORT", "DB_USER", "DB_PASS", "DB_NAME", "SERVER_PORT", "JWT_SECRET", "DATA_DIR", "LOG_LEVEL"} {
		t.Setenv(k, "")
	}
}

// The shipped example must parse, and every key in it must map to a field;
// a misplaced key is silently ignored by yaml.Unmarshal otherwise.
func TestExampleConfigHasOnlyKnownKeys(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "configs", "config.example.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	var c Config
	if err := dec.Decode(&c); err != nil {
		t.Fatalf("config.example.yaml: %v", err)
	}
	if c.Server.Port == 0 || c.Storage.Root == "" || c.JWT.Issuer == "" || c.Audit.RetainDays == 0 {
		t.Fatalf("example is missing expected values: %+v", c)
	}
}

func TestLoadAndEnvOverride(t *testing.T) {
	resetC(t)
	clearEnv(t)
	p := filepath.Join(t.TempDir(), "c.yaml")
	os.WriteFile(p, []byte("server:\n  port: 8900\ndatabase:\n  host: db\n  port: 3306\n  dbname: file_db\njwt:\n  secret: file-secret\nstorage:\n  root: /srv/files\n"), 0600)

	if err := Load(p); err != nil {
		t.Fatal(err)
	}
	if C.Server.Port != 8900 || C.Database.Host != "db" || C.JWT.Secret != "file-secret" || C.Storage.Root != "/srv/files" {
		t.Fatalf("file values not loaded: %+v", C)
	}

	resetC(t)
	t.Setenv("DB_HOST", "envhost")
	t.Setenv("DB_PORT", "3307")
	t.Setenv("DB_NAME", "env_db")
	t.Setenv("SERVER_PORT", "not-a-number")
	t.Setenv("JWT_SECRET", "env-secret")
	t.Setenv("DATA_DIR", "/data")
	if err := Load(p); err != nil {
		t.Fatal(err)
	}
	if C.Database.Host != "envhost" || C.Database.Port != 3307 || C.Database.DBName != "env_db" {
		t.Errorf("db env override: %+v", C.Database)
	}
	if C.Server.Port != 8900 {
		t.Errorf("invalid SERVER_PORT must keep the file value, got %d", C.Server.Port)
	}
	if C.JWT.Secret != "env-secret" || C.Storage.Root != "/data/files" {
		t.Errorf("secret/root override: %q %q", C.JWT.Secret, C.Storage.Root)
	}
}

func TestLoadErrors(t *testing.T) {
	resetC(t)
	if err := Load(filepath.Join(t.TempDir(), "missing.yaml")); err == nil {
		t.Fatal("missing file should error")
	}
	p := filepath.Join(t.TempDir(), "bad.yaml")
	os.WriteFile(p, []byte("server: [not a map"), 0600)
	if err := Load(p); err == nil {
		t.Fatal("invalid yaml should error")
	}
}

func TestAtoi(t *testing.T) {
	cases := map[string]int{"0": 0, "42": 42, "3306": 3306, "": 0, "-1": 7, "12a": 7}
	for in, want := range cases {
		if got := atoi(in, 7); got != want {
			t.Errorf("atoi(%q) = %d, want %d", in, got, want)
		}
	}
}
