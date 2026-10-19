package database

import (
	"database/sql"
	"os"
	"testing"
)

// TestMigrateFulltextNgramIdempotent connects to the CI MySQL (or a local
// database set via MIST_TEST_DSN) and runs migrateFulltextNgram twice to prove
// it is idempotent and leaves an ngram index behind.
//
// Skipped when MIST_TEST_DSN is not set (unit-only runs).
func TestMigrateFulltextNgramIdempotent(t *testing.T) {
	dsn := os.Getenv("MIST_TEST_DSN")
	if dsn == "" {
		t.Skip("MIST_TEST_DSN not set; skipping DB integration test")
	}
	if DB != nil {
		defer func(old *sql.DB) { DB = old }(DB)
	} else {
		defer func() { DB = nil }()
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		t.Fatalf("ping test db: %v", err)
	}
	DB = db

	// First run creates the index; migrateFulltextNgram swallows and logs the
	// duplicate-name case internally, so it has no error return by design.
	migrateFulltextNgram()
	// Second run must be a no-op (duplicate key name is swallowed internally).
	migrateFulltextNgram()

	var count int
	if err := DB.QueryRow(`SELECT COUNT(*) FROM information_schema.statistics
		WHERE table_schema = DATABASE() AND table_name = 'md_documents'
		AND index_name = 'ft_content_ngram'`).Scan(&count); err != nil {
		t.Fatalf("lookup ft_content_ngram: %v", err)
	}
	if count == 0 {
		t.Fatalf("ft_content_ngram missing after migrateFulltextNgram")
	}

	// HasNgram must be true so search binds MATCH() to the ngram index.
	if !HasNgram() {
		t.Fatalf("HasNgram() = false after successful index creation")
	}
}
