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

func TestContentSnippetPreservesYearAtLineStart(t *testing.T) {
	// Bug fix: previously, "2026 was a great year." had "2026 " stripped as a
	// fake numbered-list marker. It should be preserved.
	got := contentSnippet("2026 was a great year.")
	if !strings.Contains(got, "2026") {
		t.Errorf("got %q, should retain '2026'", got)
	}
}

func TestContentSnippetStripsNumberedListMarkers(t *testing.T) {
	got := contentSnippet("1. First item\n2. Second item")
	if strings.Contains(got, "1.") || strings.Contains(got, "2.") {
		t.Errorf("got %q, should strip list markers", got)
	}
	if !strings.Contains(got, "First item") || !strings.Contains(got, "Second item") {
		t.Errorf("got %q, should retain list content", got)
	}
}

func TestContentSnippetHardCutWhenNoSpaces(t *testing.T) {
	// Exercises the fall-through branch when the 160-char window has no space.
	long := strings.Repeat("a", 200)
	got := contentSnippet(long)
	if len(got) != 160 {
		t.Errorf("len(got) = %d, want exactly 160 (hard cut)", len(got))
	}
}
