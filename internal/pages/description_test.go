package pages

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
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

type stubClient struct {
	response string
	err      error
}

func (s *stubClient) CreateMessage(ctx context.Context, apiKey string, req ClaudeRequest) (string, error) {
	return s.response, s.err
}

func TestGeneratorUsesAIResult(t *testing.T) {
	db, _ := setupTestDB(t)
	db.Exec(`INSERT INTO settings (key, value) VALUES ('ai_api_key', 'test-key')`)

	gen := &Generator{db: db, client: &stubClient{response: "A thoughtful summary of the page in exactly one sentence."}, timeout: time.Second}
	got := gen.Generate(context.Background(), "Title", "Body content.")
	if got != "A thoughtful summary of the page in exactly one sentence." {
		t.Errorf("got %q, want AI response", got)
	}
}

func TestGeneratorFallsBackOnClientError(t *testing.T) {
	db, _ := setupTestDB(t)
	db.Exec(`INSERT INTO settings (key, value) VALUES ('ai_api_key', 'test-key')`)

	gen := &Generator{db: db, client: &stubClient{err: errors.New("api down")}, timeout: time.Second}
	got := gen.Generate(context.Background(), "Title", "Body content here.")
	if got == "" {
		t.Fatal("expected non-empty fallback")
	}
	if got != "Body content here." {
		t.Errorf("got %q, want fallback content snippet", got)
	}
}

func TestGeneratorFallsBackWhenNoAPIKey(t *testing.T) {
	db, _ := setupTestDB(t)
	// No ai_api_key row inserted.

	gen := &Generator{db: db, client: &stubClient{response: "should not be called"}, timeout: time.Second}
	got := gen.Generate(context.Background(), "Title", "Body content here.")
	if got != "Body content here." {
		t.Errorf("got %q, want fallback", got)
	}
}

func TestGeneratorTrimsQuotesAndWhitespace(t *testing.T) {
	db, _ := setupTestDB(t)
	db.Exec(`INSERT INTO settings (key, value) VALUES ('ai_api_key', 'test-key')`)

	gen := &Generator{db: db, client: &stubClient{response: `  "Hello world."  `}, timeout: time.Second}
	got := gen.Generate(context.Background(), "Title", "Body.")
	if got != "Hello world." {
		t.Errorf("got %q, want trimmed", got)
	}
}

func TestGeneratorCaps160(t *testing.T) {
	db, _ := setupTestDB(t)
	db.Exec(`INSERT INTO settings (key, value) VALUES ('ai_api_key', 'test-key')`)

	long := strings.Repeat("word ", 60) // 300 chars
	gen := &Generator{db: db, client: &stubClient{response: long}, timeout: time.Second}
	got := gen.Generate(context.Background(), "Title", "Body.")
	if len(got) > 160 {
		t.Errorf("len(got) = %d, want ≤ 160", len(got))
	}
}
