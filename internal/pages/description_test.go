package pages

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
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
	response   string
	err        error
	lastSystem string // captured from most recent call
}

func (s *stubClient) CreateMessage(ctx context.Context, apiKey string, req ClaudeRequest) (string, error) {
	s.lastSystem = req.System
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

func TestHandlerUpdatePageManualTriggersGeneration(t *testing.T) {
	db, contentDir := setupTestDB(t)
	db.Exec(`INSERT INTO settings (key, value) VALUES ('ai_api_key', 'test-key')`)

	svc := NewService(db, contentDir)
	if err := svc.CreatePage("hello", "Hello", "# Hi\n\nOriginal body."); err != nil {
		t.Fatalf("CreatePage: %v", err)
	}

	gen := &Generator{db: db, client: &stubClient{response: "Generated description."}, timeout: time.Second}
	h := &Handler{svc: svc, baseURL: "https://example.com", descGen: gen}

	trueVal := true
	body, _ := json.Marshal(PageRequest{Title: "Hello", Content: "# Hi\n\nUpdated body.", Manual: &trueVal})
	req := httptest.NewRequest("PUT", "/api/pages/hello", bytes.NewReader(body))
	req.SetPathValue("path", "hello")
	w := httptest.NewRecorder()
	h.UpdatePage(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	var desc string
	db.QueryRow(`SELECT description FROM pages WHERE path = 'hello'`).Scan(&desc)
	if desc != "Generated description." {
		t.Errorf("description = %q, want %q", desc, "Generated description.")
	}
}

func TestHandlerUpdatePageAutoSaveSkipsGeneration(t *testing.T) {
	db, contentDir := setupTestDB(t)
	db.Exec(`INSERT INTO settings (key, value) VALUES ('ai_api_key', 'test-key')`)

	svc := NewService(db, contentDir)
	svc.CreatePage("hello", "Hello", "Body.")
	// Prepopulate description
	db.Exec(`UPDATE pages SET description = 'pre-existing' WHERE path = 'hello'`)

	gen := &Generator{db: db, client: &stubClient{response: "SHOULD NOT APPEAR"}, timeout: time.Second}
	h := &Handler{svc: svc, baseURL: "https://example.com", descGen: gen}

	body, _ := json.Marshal(PageRequest{Title: "Hello", Content: "Updated body."}) // No Manual
	req := httptest.NewRequest("PUT", "/api/pages/hello", bytes.NewReader(body))
	req.SetPathValue("path", "hello")
	w := httptest.NewRecorder()
	h.UpdatePage(w, req)

	var desc string
	db.QueryRow(`SELECT description FROM pages WHERE path = 'hello'`).Scan(&desc)
	if desc != "pre-existing" {
		t.Errorf("description changed to %q, should have stayed 'pre-existing'", desc)
	}
}

func TestBackfillEmptyPopulatesDescriptions(t *testing.T) {
	db, contentDir := setupTestDB(t)
	db.Exec(`INSERT INTO settings (key, value) VALUES ('ai_api_key', 'test-key')`)

	svc := NewService(db, contentDir)
	svc.CreatePage("p1", "P1", "Body one.")
	svc.CreatePage("p2", "P2", "Body two.")
	// Prepopulate one, leave the other empty
	db.Exec(`UPDATE pages SET description = 'already set' WHERE path = 'p1'`)

	gen := &Generator{
		db:      db,
		client:  &stubClient{response: "AI summary"},
		timeout: time.Second,
	}

	// Only one page has empty description; backfill runs one iteration + one
	// 500ms sleep at the end before the loop re-queries and exits via
	// sql.ErrNoRows. Total runtime ~500ms.
	gen.BackfillEmpty(context.Background(), svc)

	var d1, d2 string
	db.QueryRow(`SELECT description FROM pages WHERE path = 'p1'`).Scan(&d1)
	db.QueryRow(`SELECT description FROM pages WHERE path = 'p2'`).Scan(&d2)

	if d1 != "already set" {
		t.Errorf("p1 description = %q, want unchanged", d1)
	}
	if d2 != "AI summary" {
		t.Errorf("p2 description = %q, want generated", d2)
	}
}

func TestBackfillRespectsContext(t *testing.T) {
	db, contentDir := setupTestDB(t)
	db.Exec(`INSERT INTO settings (key, value) VALUES ('ai_api_key', 'test-key')`)

	svc := NewService(db, contentDir)
	for i := 0; i < 5; i++ {
		svc.CreatePage(fmt.Sprintf("p%d", i), fmt.Sprintf("P%d", i), "Body.")
	}

	gen := &Generator{
		db:      db,
		client:  &stubClient{response: "summary"},
		timeout: time.Second,
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Pre-canceled

	start := time.Now()
	gen.BackfillEmpty(ctx, svc)
	elapsed := time.Since(start)

	if elapsed > 100*time.Millisecond {
		t.Errorf("backfill took %v, should have exited immediately on canceled ctx", elapsed)
	}
}

func TestGeneratorUsesCustomDescriptionPromptFromDB(t *testing.T) {
	db, _ := setupTestDB(t)
	db.Exec(`INSERT INTO settings (key, value) VALUES ('ai_api_key', 'test-key')`)
	db.Exec(`INSERT INTO settings (key, value) VALUES ('ai_description_prompt', 'Custom prompt here.')`)

	stub := &stubClient{response: "A thoughtful summary of the page."}
	gen := &Generator{db: db, client: stub, timeout: time.Second}
	gen.Generate(context.Background(), "Title", "Body content.")

	if stub.lastSystem != "Custom prompt here." {
		t.Errorf("System = %q, want %q", stub.lastSystem, "Custom prompt here.")
	}
}

func TestBackfillRecoverFromBrokenPage(t *testing.T) {
	db, contentDir := setupTestDB(t)
	db.Exec(`INSERT INTO settings (key, value) VALUES ('ai_api_key', 'test-key')`)

	svc := NewService(db, contentDir)
	svc.CreatePage("p1", "P1", "Body.")
	// Simulate a broken page: delete the underlying file so GetPage fails,
	// but leave the DB row with empty description intact.
	if err := os.Remove(filepath.Join(contentDir, "p1.md")); err != nil {
		t.Fatalf("remove md file: %v", err)
	}

	gen := &Generator{db: db, client: &stubClient{response: "should not be used"}, timeout: time.Second}

	// Give the loop enough room to process + pause. With one broken page and
	// no subsequent empty rows, it should mark-and-continue, then exit on
	// ErrNoRows. Bound total runtime.
	done := make(chan struct{})
	go func() {
		gen.BackfillEmpty(context.Background(), svc)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("BackfillEmpty did not return within 2s")
	}

	var desc string
	db.QueryRow(`SELECT description FROM pages WHERE path = 'p1'`).Scan(&desc)
	if desc != " " {
		t.Errorf("description = %q, want %q (mark-with-space sentinel)", desc, " ")
	}
}

func TestGeneratorUsesDefaultWhenDescriptionPromptUnset(t *testing.T) {
	db, _ := setupTestDB(t)
	db.Exec(`INSERT INTO settings (key, value) VALUES ('ai_api_key', 'test-key')`)
	// ai_description_prompt not inserted.

	stub := &stubClient{response: "ignored"}
	gen := &Generator{db: db, client: stub, timeout: time.Second}
	gen.Generate(context.Background(), "Title", "Body.")

	if stub.lastSystem != DefaultDescriptionPrompt {
		t.Errorf("System = %q, want DefaultDescriptionPrompt", stub.lastSystem)
	}
}

func TestGeneratorUsesDefaultWhenDescriptionPromptEmpty(t *testing.T) {
	db, _ := setupTestDB(t)
	db.Exec(`INSERT INTO settings (key, value) VALUES ('ai_api_key', 'test-key')`)
	db.Exec(`INSERT INTO settings (key, value) VALUES ('ai_description_prompt', '   ')`) // whitespace only

	stub := &stubClient{response: "ignored"}
	gen := &Generator{db: db, client: stub, timeout: time.Second}
	gen.Generate(context.Background(), "Title", "Body.")

	if stub.lastSystem != DefaultDescriptionPrompt {
		t.Errorf("System = %q, want DefaultDescriptionPrompt (whitespace-only should fall back)", stub.lastSystem)
	}
}

func TestGeneratorUsesDefaultWhenDescriptionPromptLiteralEmpty(t *testing.T) {
	// Covers the reset-to-default path: the UI writes "" (empty string) to
	// the DB, which must fall back to the default at generation time.
	db, _ := setupTestDB(t)
	db.Exec(`INSERT INTO settings (key, value) VALUES ('ai_api_key', 'test-key')`)
	db.Exec(`INSERT INTO settings (key, value) VALUES ('ai_description_prompt', '')`)

	stub := &stubClient{response: "ignored"}
	gen := &Generator{db: db, client: stub, timeout: time.Second}
	gen.Generate(context.Background(), "Title", "Body.")

	if stub.lastSystem != DefaultDescriptionPrompt {
		t.Errorf("System = %q, want DefaultDescriptionPrompt (literal empty string should fall back)", stub.lastSystem)
	}
}
