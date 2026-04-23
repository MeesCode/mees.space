package main

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"

	"mees.space/internal/pages"
	"mees.space/internal/seo"
)

func setupContentPageTest(t *testing.T) (*pages.Service, *seo.Injector) {
	t.Helper()
	tmp := t.TempDir()

	dbPath := filepath.Join(tmp, "test.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	_, err = db.Exec(`CREATE TABLE pages (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		path TEXT NOT NULL UNIQUE,
		title TEXT NOT NULL,
		view_count INTEGER NOT NULL DEFAULT 0,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		show_date BOOLEAN NOT NULL DEFAULT 0,
		published BOOLEAN NOT NULL DEFAULT 1,
		description TEXT NOT NULL DEFAULT ''
	)`)
	if err != nil {
		t.Fatal(err)
	}

	contentDir := filepath.Join(tmp, "content")
	if err := os.MkdirAll(contentDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(contentDir, "home.md"), []byte("# Home\n\nBody."), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO pages (path, title) VALUES ('home', 'Home Page')`); err != nil {
		t.Fatal(err)
	}

	distDir := filepath.Join(tmp, "dist")
	if err := os.MkdirAll(distDir, 0755); err != nil {
		t.Fatal(err)
	}
	shell := "<!doctype html><html><head><title>Default</title></head><body></body></html>"
	if err := os.WriteFile(filepath.Join(distDir, "index.html"), []byte(shell), 0644); err != nil {
		t.Fatal(err)
	}

	injector, err := seo.NewInjector(distDir)
	if err != nil {
		t.Fatal(err)
	}
	svc := pages.NewService(db, contentDir)
	return svc, injector
}

func TestServeContentPageFound(t *testing.T) {
	svc, injector := setupContentPageTest(t)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/home", nil)
	serveContentPage(w, r, "home", "https://example.com", svc, injector)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "<title>Home Page</title>") {
		t.Errorf("body missing page title; got:\n%s", body)
	}
}

func TestServeContentPageNotFound(t *testing.T) {
	svc, injector := setupContentPageTest(t)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/does-not-exist", nil)
	serveContentPage(w, r, "does-not-exist", "https://example.com", svc, injector)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, `<meta name="robots" content="noindex">`) {
		t.Errorf("body missing noindex meta; got:\n%s", body)
	}
	if !strings.Contains(body, "Not Found — Mees Brinkhuis") {
		t.Errorf("body missing generic Not Found title; got:\n%s", body)
	}
}
