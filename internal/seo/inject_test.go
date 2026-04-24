package seo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const shellWithMarkers = `<html>
<head></head>
<body>
<div id="content"><!--SSR_CONTENT--></div>
<!--SSR_DATA-->
</body>
</html>`

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

func TestNewInjectorMissingContentMarker(t *testing.T) {
	dir := writeTestHTML(t, `<html><head></head><body><!--SSR_DATA--></body></html>`)
	_, err := NewInjector(dir)
	if err == nil {
		t.Fatal("expected error when SSR_CONTENT marker is missing")
	}
}

func TestNewInjectorMissingDataMarker(t *testing.T) {
	dir := writeTestHTML(t, `<html><head></head><body><div id="content"><!--SSR_CONTENT--></div></body></html>`)
	_, err := NewInjector(dir)
	if err == nil {
		t.Fatal("expected error when SSR_DATA marker is missing")
	}
}

func TestInjectorInjectsAllTags(t *testing.T) {
	dir := writeTestHTML(t, shellWithMarkers)
	inj, err := NewInjector(dir)
	if err != nil {
		t.Fatal(err)
	}
	got := string(inj.Inject(PageMeta{
		Title:        "My Page",
		Description:  "A great page.",
		CanonicalURL: "https://example.com/my-page",
		OGImage:      "https://example.com/img.png",
	}, BodyInjection{}))

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
	dir := writeTestHTML(t, shellWithMarkers)
	inj, _ := NewInjector(dir)

	got := string(inj.Inject(PageMeta{Title: "T", NoIndex: true}, BodyInjection{}))
	if !strings.Contains(got, `<meta name="robots" content="noindex">`) {
		t.Error("missing noindex meta")
	}

	got2 := string(inj.Inject(PageMeta{Title: "T", NoIndex: false}, BodyInjection{}))
	if strings.Contains(got2, "noindex") {
		t.Error("should not include noindex when NoIndex is false")
	}
}

func TestInjectorEscapesValues(t *testing.T) {
	dir := writeTestHTML(t, shellWithMarkers)
	inj, _ := NewInjector(dir)

	got := string(inj.Inject(PageMeta{
		Title:       `Hello "<script>alert(1)</script>"`,
		Description: `AT&T says "hi"`,
	}, BodyInjection{}))
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
	dir := writeTestHTML(t, shellWithMarkers)
	inj, _ := NewInjector(dir)

	expected := strings.ReplaceAll(strings.ReplaceAll(shellWithMarkers, "<!--SSR_CONTENT-->", ""), "<!--SSR_DATA-->", "")
	got := string(inj.Raw())
	if got != expected {
		t.Errorf("Raw() should return shell with markers stripped\ngot:  %q\nwant: %q", got, expected)
	}
}

func TestInjectorBodyHTMLAndData(t *testing.T) {
	dir := writeTestHTML(t, shellWithMarkers)
	inj, err := NewInjector(dir)
	if err != nil {
		t.Fatal(err)
	}
	got := string(inj.Inject(PageMeta{Title: "T"}, BodyInjection{
		HTML: []byte(`<h2 id="x">X</h2>`),
		Data: []byte(`{"path":"home"}`),
	}))

	if !strings.Contains(got, `<div id="content"><h2 id="x">X</h2></div>`) {
		t.Errorf("content not injected, got:\n%s", got)
	}
	if !strings.Contains(got, `<script id="__page_data__" type="application/json">{"path":"home"}</script>`) {
		t.Errorf("data script not injected, got:\n%s", got)
	}
	if strings.Contains(got, "SSR_CONTENT") || strings.Contains(got, "SSR_DATA") {
		t.Errorf("markers not stripped, got:\n%s", got)
	}
}

func TestInjectorBodyEmptyStripsMarkers(t *testing.T) {
	dir := writeTestHTML(t, shellWithMarkers)
	inj, _ := NewInjector(dir)
	got := string(inj.Inject(PageMeta{Title: "T"}, BodyInjection{}))
	if strings.Contains(got, "SSR_CONTENT") || strings.Contains(got, "SSR_DATA") {
		t.Errorf("markers should be stripped even with empty body, got:\n%s", got)
	}
	if strings.Contains(got, `<script id="__page_data__"`) {
		t.Errorf("data script should not appear with empty Data, got:\n%s", got)
	}
}

func TestInjectorRawStripsMarkers(t *testing.T) {
	dir := writeTestHTML(t, shellWithMarkers)
	inj, _ := NewInjector(dir)
	got := string(inj.Raw())
	if strings.Contains(got, "SSR_CONTENT") || strings.Contains(got, "SSR_DATA") {
		t.Errorf("Raw() must strip markers, got:\n%s", got)
	}
}
