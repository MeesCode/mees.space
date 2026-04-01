package database

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpenAndMigrate(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	migrationsDir := filepath.Join(tmpDir, "migrations")
	os.MkdirAll(migrationsDir, 0755)

	os.WriteFile(filepath.Join(migrationsDir, "001_create_users.up.sql"), []byte(`
		CREATE TABLE users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
	`), 0644)
	os.WriteFile(filepath.Join(migrationsDir, "001_create_users.down.sql"), []byte(`DROP TABLE IF EXISTS users;`), 0644)
	os.WriteFile(filepath.Join(migrationsDir, "002_create_pages.up.sql"), []byte(`
		CREATE TABLE pages (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			path TEXT NOT NULL UNIQUE,
			title TEXT NOT NULL,
			view_count INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
		CREATE INDEX idx_pages_path ON pages(path);
	`), 0644)
	os.WriteFile(filepath.Join(migrationsDir, "002_create_pages.down.sql"), []byte(`DROP TABLE IF EXISTS pages;`), 0644)

	if err := Migrate(db, migrationsDir); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	var tableName string
	err = db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='users'").Scan(&tableName)
	if err != nil {
		t.Fatal("users table not found")
	}

	err = db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='pages'").Scan(&tableName)
	if err != nil {
		t.Fatal("pages table not found")
	}

	var indexName string
	err = db.QueryRow("SELECT name FROM sqlite_master WHERE type='index' AND name='idx_pages_path'").Scan(&indexName)
	if err != nil {
		t.Fatal("idx_pages_path index not found")
	}

	// Running migrate again should be no-op
	if err := Migrate(db, migrationsDir); err != nil {
		t.Fatalf("Migrate (second run): %v", err)
	}
}
