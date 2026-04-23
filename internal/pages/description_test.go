package pages

import (
	"strings"
	"testing"
)

func TestContentSnippetPlainProse(t *testing.T) {
	got := contentSnippet("This is a short piece of prose that should be returned mostly as-is.")
	want := "This is a short piece of prose that should be returned mostly as-is."
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestContentSnippetStripsMarkdownHeaders(t *testing.T) {
	got := contentSnippet("# A Post\n\n## Section\n\nHere is the actual content of the post.")
	if strings.Contains(got, "#") {
		t.Errorf("got %q, should not contain '#'", got)
	}
	if !strings.Contains(got, "Here is the actual content") {
		t.Errorf("got %q, should contain prose text", got)
	}
}

func TestContentSnippetStripsFencedCode(t *testing.T) {
	in := "Intro text.\n\n```go\nfunc main() { println(\"hello\") }\n```\n\nOutro text."
	got := contentSnippet(in)
	if strings.Contains(got, "func main") {
		t.Errorf("got %q, should not contain code block contents", got)
	}
	if !strings.Contains(got, "Intro text") {
		t.Errorf("got %q, should retain prose", got)
	}
}

func TestContentSnippetStripsInlineMarkdown(t *testing.T) {
	got := contentSnippet("This has *italic* and **bold** and `code` and [a link](http://x).")
	if strings.ContainsAny(got, "*`") {
		t.Errorf("got %q, should not contain inline markdown syntax", got)
	}
	if !strings.Contains(got, "a link") {
		t.Errorf("got %q, should keep link text", got)
	}
}

func TestContentSnippetCapAt160(t *testing.T) {
	long := strings.Repeat("word ", 100) // 500 chars
	got := contentSnippet(long)
	if len(got) > 160 {
		t.Errorf("len(got) = %d, want ≤ 160", len(got))
	}
}

func TestContentSnippetEmptyInput(t *testing.T) {
	got := contentSnippet("")
	if got != "" {
		t.Errorf("got %q, want empty", got)
	}
}
