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
	"mees.space/internal/render"
	"mees.space/internal/seo"
)

func setupContentPageTest(t *testing.T) (*pages.Service, *seo.Injector, *render.Renderer) {
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

	// Seed a draft page (published = 0) with unique sentinel strings so tests
	// can verify they are NOT leaked into the served response.
	if err := os.WriteFile(filepath.Join(contentDir, "draft-page.md"), []byte("# Secret Draft\n\nUnreleased body text."), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO pages (path, title, published) VALUES ('draft-page', 'Secret Draft', 0)`); err != nil {
		t.Fatal(err)
	}

	distDir := filepath.Join(tmp, "dist")
	if err := os.MkdirAll(distDir, 0755); err != nil {
		t.Fatal(err)
	}
	shell := "<!doctype html><html><head><title>Default</title></head><body><!--SSR_CONTENT--><!--SSR_DATA--></body></html>"
	if err := os.WriteFile(filepath.Join(distDir, "index.html"), []byte(shell), 0644); err != nil {
		t.Fatal(err)
	}

	injector, err := seo.NewInjector(distDir)
	if err != nil {
		t.Fatal(err)
	}
	svc := pages.NewService(db, contentDir)
	renderer := render.New()
	return svc, injector, renderer
}

func TestServeContentPageFound(t *testing.T) {
	svc, injector, renderer := setupContentPageTest(t)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/home", nil)
	serveContentPage(w, r, "home", "https://example.com", svc, injector, renderer)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	body := w.Body.String()

	// Basic title injection.
	mustContain(t, body, "<title>Home Page</title>")

	// Heading with stable id from the goldmark renderer (content: "# Home").
	mustContain(t, body, `id="home"`)
	mustContain(t, body, `class="heading-anchor"`)
	mustContain(t, body, `href="#home"`)
	mustContain(t, body, `aria-hidden="true"`)
	mustContain(t, body, `tabindex="-1"`)

	// Bootstrap script injected by the seo.Injector.
	mustContain(t, body, `<script id="__page_data__" type="application/json">`)
	mustContain(t, body, `"rendered_html":`)

	// Marker comments must be consumed (not left verbatim in output).
	mustNotContain(t, body, "SSR_CONTENT")
	mustNotContain(t, body, "SSR_DATA")
}

func TestServeContentPageDraft(t *testing.T) {
	// GetPage does NOT filter by published; it returns all pages. The
	// serveContentPage function detects page.Published == false and renders a
	// safe shell: noindex meta, generic title "Draft — Mees Brinkhuis", and NO
	// body HTML / bootstrap script so that draft details are not leaked.

	svc, injector, renderer := setupContentPageTest(t)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/draft-page", nil)
	serveContentPage(w, r, "draft-page", "https://example.com", svc, injector, renderer)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 (draft pages are served, just without content)", w.Code)
	}
	body := w.Body.String()

	// Must signal to crawlers that this page should not be indexed.
	mustContain(t, body, `<meta name="robots" content="noindex">`)

	// Must use the generic draft title, not the page's real title.
	mustContain(t, body, "<title>Draft — Mees Brinkhuis</title>")

	// Must NOT expose any draft-specific content.
	mustNotContain(t, body, "Secret Draft")
	mustNotContain(t, body, "Unreleased body text.")

	// Must NOT inject the bootstrap script (would expose the draft payload).
	mustNotContain(t, body, `<script id="__page_data__"`)

	// Marker comments must be consumed.
	mustNotContain(t, body, "SSR_CONTENT")
	mustNotContain(t, body, "SSR_DATA")
}

func TestServeContentPageNotFound(t *testing.T) {
	svc, injector, renderer := setupContentPageTest(t)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/does-not-exist", nil)
	serveContentPage(w, r, "does-not-exist", "https://example.com", svc, injector, renderer)

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

// mustContain fails the test if body does not contain sub.
func mustContain(t *testing.T, body, sub string) {
	t.Helper()
	if !strings.Contains(body, sub) {
		t.Errorf("body missing %q; got:\n%s", sub, body)
	}
}

// mustNotContain fails the test if body contains sub.
func mustNotContain(t *testing.T, body, sub string) {
	t.Helper()
	if strings.Contains(body, sub) {
		t.Errorf("body must not contain %q; got:\n%s", sub, body)
	}
}
