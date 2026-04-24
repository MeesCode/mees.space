package render

import (
	"strings"
	"testing"
)

func TestToHTMLHeadingAnchor(t *testing.T) {
	r := New()
	out, err := r.ToHTML([]byte("## Hello World\n\nbody\n"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	for _, want := range []string{
		`<h2 id="hello-world">`,
		`<a href="#hello-world"`,
		`class="heading-anchor"`,
		`aria-hidden="true"`,
		`tabindex="-1"`,
		`#</a>`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("output missing %q\n---\n%s\n---", want, s)
		}
	}
}

func TestToHTMLDuplicateHeadings(t *testing.T) {
	r := New()
	out, err := r.ToHTML([]byte("## Foo\n\n## Foo\n\n## Foo\n"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	for _, want := range []string{`id="foo"`, `id="foo-1"`, `id="foo-2"`} {
		if !strings.Contains(s, want) {
			t.Errorf("output missing %q", want)
		}
	}
}

func TestToHTMLGFMTable(t *testing.T) {
	r := New()
	out, err := r.ToHTML([]byte("| a | b |\n| - | - |\n| 1 | 2 |\n"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "<table>") {
		t.Errorf("expected GFM table output, got:\n%s", out)
	}
}

func TestToHTMLRawHTMLPassthrough(t *testing.T) {
	r := New()
	out, err := r.ToHTML([]byte(`<div class="callout">hi</div>`))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), `<div class="callout">hi</div>`) {
		t.Errorf("expected raw HTML pass-through, got:\n%s", out)
	}
}
