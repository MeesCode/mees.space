package pages

import (
	"strings"
	"testing"
)

func TestBuildSitemapIncludesPublishedPages(t *testing.T) {
	db, _ := setupTestDB(t)
	db.Exec(`INSERT INTO pages (path, title, created_at, updated_at, published) VALUES ('home', 'Home', '2026-01-15T10:00:00Z', '2026-02-01T10:00:00Z', 1)`)
	db.Exec(`INSERT INTO pages (path, title, created_at, updated_at, published) VALUES ('blog/post-1', 'Post 1', '2026-01-16T10:00:00Z', '2026-01-16T10:00:00Z', 1)`)

	got, err := BuildSitemap(db, "https://example.com")
	if err != nil {
		t.Fatalf("BuildSitemap: %v", err)
	}
	s := string(got)

	if !strings.Contains(s, "<urlset") {
		t.Error("missing <urlset>")
	}
	if !strings.Contains(s, "<loc>https://example.com</loc>") {
		t.Error("home should render as bare baseURL")
	}
	if !strings.Contains(s, "<loc>https://example.com/blog/post-1</loc>") {
		t.Error("post-1 should render with path appended")
	}
	if !strings.Contains(s, "<lastmod>2026-02-01</lastmod>") {
		t.Error("home lastmod date missing")
	}
	if !strings.Contains(s, `<?xml version="1.0" encoding="UTF-8"?>`) {
		t.Error("missing XML declaration")
	}
}

func TestBuildSitemapExcludesUnpublished(t *testing.T) {
	db, _ := setupTestDB(t)
	db.Exec(`INSERT INTO pages (path, title, created_at, updated_at, published) VALUES ('draft', 'Draft', '2026-01-01T10:00:00Z', '2026-01-01T10:00:00Z', 0)`)
	db.Exec(`INSERT INTO pages (path, title, created_at, updated_at, published) VALUES ('live', 'Live', '2026-01-02T10:00:00Z', '2026-01-02T10:00:00Z', 1)`)

	got, err := BuildSitemap(db, "https://example.com")
	if err != nil {
		t.Fatalf("BuildSitemap: %v", err)
	}
	s := string(got)

	if strings.Contains(s, "draft") {
		t.Error("draft should be excluded from sitemap")
	}
	if !strings.Contains(s, "/live") {
		t.Error("published page missing")
	}
}

func TestBuildSitemapEmpty(t *testing.T) {
	db, _ := setupTestDB(t)

	got, err := BuildSitemap(db, "https://example.com")
	if err != nil {
		t.Fatalf("BuildSitemap: %v", err)
	}
	s := string(got)

	if !strings.Contains(s, "<urlset") {
		t.Error("empty sitemap still needs a <urlset>")
	}
}
