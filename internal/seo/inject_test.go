package seo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTestHTML(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestNewInjectorMissingFile(t *testing.T) {
	_, err := NewInjector(t.TempDir())
	if err == nil {
		t.Fatal("expected error when index.html is missing")
	}
}

func TestNewInjectorMissingHeadAnchor(t *testing.T) {
	dir := writeTestHTML(t, "<html><body>No head tag</body></html>")
	_, err := NewInjector(dir)
	if err == nil {
		t.Fatal("expected error when </head> anchor is missing")
	}
}

func TestInjectorInjectsAllTags(t *testing.T) {
	dir := writeTestHTML(t, "<html><head><title>Default</title></head><body></body></html>")
	inj, err := NewInjector(dir)
	if err != nil {
		t.Fatal(err)
	}
	got := string(inj.Inject(PageMeta{
		Title:        "My Page",
		Description:  "A great page.",
		CanonicalURL: "https://example.com/my-page",
		OGImage:      "https://example.com/img.png",
	}))

	expected := []string{
		`<title>My Page</title>`,
		`<meta name="description" content="A great page.">`,
		`<link rel="canonical" href="https://example.com/my-page">`,
		`<meta property="og:title" content="My Page">`,
		`<meta property="og:description" content="A great page.">`,
		`<meta property="og:url" content="https://example.com/my-page">`,
		`<meta property="og:image" content="https://example.com/img.png">`,
		`<meta property="og:type" content="article">`,
		`<meta name="twitter:card" content="summary_large_image">`,
		`<meta name="twitter:title" content="My Page">`,
		`<meta name="twitter:description" content="A great page.">`,
		`<meta name="twitter:image" content="https://example.com/img.png">`,
	}
	for _, e := range expected {
		if !strings.Contains(got, e) {
			t.Errorf("injected HTML missing %q", e)
		}
	}
}

func TestInjectorNoIndex(t *testing.T) {
	dir := writeTestHTML(t, "<html><head></head></html>")
	inj, _ := NewInjector(dir)

	got := string(inj.Inject(PageMeta{Title: "T", NoIndex: true}))
	if !strings.Contains(got, `<meta name="robots" content="noindex">`) {
		t.Error("missing noindex meta")
	}

	got2 := string(inj.Inject(PageMeta{Title: "T", NoIndex: false}))
	if strings.Contains(got2, "noindex") {
		t.Error("should not include noindex when NoIndex is false")
	}
}

func TestInjectorEscapesValues(t *testing.T) {
	dir := writeTestHTML(t, "<html><head></head></html>")
	inj, _ := NewInjector(dir)

	got := string(inj.Inject(PageMeta{
		Title:       `Hello "<script>alert(1)</script>"`,
		Description: `AT&T says "hi"`,
	}))
	if strings.Contains(got, "<script>alert") {
		t.Error("unescaped script tag in output — XSS risk")
	}
	if !strings.Contains(got, "&lt;script&gt;") {
		t.Error("expected HTML-escaped script")
	}
	if !strings.Contains(got, "AT&amp;T") {
		t.Error("expected ampersand escaping")
	}
}

func TestInjectorRaw(t *testing.T) {
	body := "<html><head></head><body>hello</body></html>"
	dir := writeTestHTML(t, body)
	inj, _ := NewInjector(dir)

	if string(inj.Raw()) != body {
		t.Error("Raw() should return the unmodified template")
	}
}
