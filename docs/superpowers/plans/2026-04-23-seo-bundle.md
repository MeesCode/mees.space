# SEO Bundle Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship real SEO for the site: sitemap.xml, robots.txt, per-page OG/meta/canonical tags injected into the initial HTML by the Go backend, and AI-generated page descriptions populated on manual save with a content-derived fallback.

**Architecture:** The Go backend already owns page metadata in SQLite and serves `dist/*` via a static handler. A new `internal/seo` package reads `dist/index.html` once at startup and injects per-page `<head>` tags via a single `bytes.Replace` anchored on `</head>` before `staticHandler` serves content pages. Descriptions come from a new `pages.Generator` that calls Claude Haiku via the existing `settings.ai_api_key` and falls back to a markdown-stripped content snippet on any error. One new DB migration adds a `description` column. The Next.js static export is untouched.

**Tech Stack:** Go 1.26+ (std lib net/http, html/template escaping, database/sql via modernc.org/sqlite). Frontend: Next.js 15 static export, React 19, TypeScript. Claude API (Haiku 4.5 only, hit directly with `net/http` — no SDK).

**Reference spec:** `docs/superpowers/specs/2026-04-23-seo-bundle-design.md`

**Notes for implementers:**
- The project uses Node 20 via nvm; the default zsh shell does NOT persist nvm between bash tool calls. Any `npm` command must be wrapped: `bash -c 'export NVM_DIR="$HOME/.nvm"; source "$NVM_DIR/nvm.sh"; nvm use 20 > /dev/null; cd <path>; <command>'`.
- Backend Go tests use `go test ./...` from repo root. Tests use `modernc.org/sqlite` with a tmpdir-backed DB.
- The repo's `setupTestDB` helper (in `internal/pages/service_test.go`) creates the pages table inline with the CURRENT schema. When this plan adds the `description` column (Task 3), that helper must be updated so existing tests keep compiling.
- The existing `internal/ai/handler.go` loads `ai_api_key` from the `settings` table per request. The new description generator follows the same pattern.

---

## File Map

**Create:**
- `migrations/007_add_description.up.sql`
- `migrations/007_add_description.down.sql`
- `internal/pages/description.go` — content snippet fallback + AI generator
- `internal/pages/description_test.go`
- `internal/pages/sitemap.go` — `BuildSitemap`
- `internal/pages/sitemap_test.go`
- `internal/seo/inject.go` — `Injector`, `PageMeta`, `NewInjector`, `Inject`, `Raw`
- `internal/seo/inject_test.go`

**Modify:**
- `internal/config/config.go` — add `BaseURL` field
- `internal/config/config_test.go` — add default + override tests
- `.env.example` — document `MEES_BASE_URL`
- `internal/pages/model.go` — add `Description` to `PageResponse`, add `Manual *bool` to `PageRequest`
- `internal/pages/service.go` — include `description` in the `SELECT` in `GetPage`
- `internal/pages/service_test.go` — update `setupTestDB` schema to include `description`
- `internal/pages/handler.go` — new `GetSitemap`; modified `UpdatePage` to regenerate description on manual save; RSS uses injected baseURL
- `internal/pages/rss.go` — `BuildRSSFeed` signature unchanged (already takes baseURL); remove the hardcode from handler (was `baseURL := "https://mees.space"`)
- `cmd/server/main.go` — construct `seo.Injector` at startup; register `/sitemap.xml` and `/robots.txt`; wire description `Generator` into `pagesHandler`; start backfill goroutine; update `staticHandler` signature and behavior
- `frontend/src/app/admin/editor/page.tsx` — Save button label + `manual: true` in save body

---

## Task 1: Add `BaseURL` to config

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`
- Modify: `.env.example`

- [ ] **Step 1: Write the failing tests**

Append to `internal/config/config_test.go`:

```go
func TestLoadBaseURLDefault(t *testing.T) {
	os.Setenv("MEES_JWT_SECRET", "test")
	os.Unsetenv("MEES_BASE_URL")
	defer os.Unsetenv("MEES_JWT_SECRET")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.BaseURL != "https://mees.space" {
		t.Errorf("BaseURL = %q, want default %q", cfg.BaseURL, "https://mees.space")
	}
}

func TestLoadBaseURLOverride(t *testing.T) {
	os.Setenv("MEES_JWT_SECRET", "test")
	os.Setenv("MEES_BASE_URL", "http://localhost:8080")
	defer func() {
		os.Unsetenv("MEES_JWT_SECRET")
		os.Unsetenv("MEES_BASE_URL")
	}()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.BaseURL != "http://localhost:8080" {
		t.Errorf("BaseURL = %q, want override", cfg.BaseURL)
	}
}
```

- [ ] **Step 2: Run tests to verify failure**

```bash
go test ./internal/config/ -run TestLoadBaseURL -v
```
Expected: FAIL — `BaseURL` field doesn't exist.

- [ ] **Step 3: Add the field**

In `internal/config/config.go`:

Add `BaseURL string` to the `Config` struct (after `DistDir`):

```go
type Config struct {
	Port                string
	DatabasePath        string
	ContentDir          string
	UploadsDir          string
	DistDir             string
	BaseURL             string
	JWTSecret           string
	JWTExpiryMinutes    int
	JWTRefreshExpiryHrs int
	AdminPassword       string
}
```

In `Load()`, add to the initializer (after `DistDir`):

```go
BaseURL: envOr("MEES_BASE_URL", "https://mees.space"),
```

- [ ] **Step 4: Run tests to verify pass**

```bash
go test ./internal/config/ -v
```
Expected: PASS (all tests in the package).

- [ ] **Step 5: Document in `.env.example`**

Append to `.env.example`:

```
# Canonical site URL used for RSS, sitemap, and canonical/OG tags
MEES_BASE_URL=https://mees.space
```

- [ ] **Step 6: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go .env.example
git commit -m "feat(config): add MEES_BASE_URL"
```

---

## Task 2: Use `cfg.BaseURL` for RSS instead of the hardcoded string

**Files:**
- Modify: `internal/pages/handler.go` (around lines 202–213)
- Modify: `cmd/server/main.go` (pagesHandler construction)

This task threads `baseURL` through the pages handler so RSS and (later) sitemap share a source of truth.

- [ ] **Step 1: Accept baseURL in the handler constructor**

In `internal/pages/handler.go`, change the Handler struct and constructor:

```go
type Handler struct {
	svc     *Service
	baseURL string
}

func NewHandler(svc *Service, baseURL string) *Handler {
	return &Handler{svc: svc, baseURL: baseURL}
}
```

Change `GetRSS`:

```go
func (h *Handler) GetRSS(w http.ResponseWriter, r *http.Request) {
	feed, err := BuildRSSFeed(h.svc.db, h.baseURL)
	if err != nil {
		http.Error(w, "failed to build feed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/rss+xml; charset=utf-8")
	w.Write(feed)
}
```

- [ ] **Step 2: Update the constructor call in main.go**

In `cmd/server/main.go`, find the `pagesHandler := pages.NewHandler(pagesSvc)` line and change to:

```go
pagesHandler := pages.NewHandler(pagesSvc, cfg.BaseURL)
```

- [ ] **Step 3: Build and verify**

```bash
go build ./...
go test ./...
```
Expected: All passes. No existing tests break.

- [ ] **Step 4: Commit**

```bash
git add internal/pages/handler.go cmd/server/main.go
git commit -m "refactor(pages): use cfg.BaseURL for RSS"
```

---

## Task 3: Migration for `description` column + model/service updates

**Files:**
- Create: `migrations/007_add_description.up.sql`
- Create: `migrations/007_add_description.down.sql`
- Modify: `internal/pages/model.go`
- Modify: `internal/pages/service.go`
- Modify: `internal/pages/service_test.go` (update `setupTestDB` schema)

- [ ] **Step 1: Create migration files**

`migrations/007_add_description.up.sql`:

```sql
ALTER TABLE pages ADD COLUMN description TEXT NOT NULL DEFAULT '';
```

`migrations/007_add_description.down.sql`:

```sql
ALTER TABLE pages DROP COLUMN description;
```

- [ ] **Step 2: Add `Description` to `PageResponse` and `Manual` to `PageRequest`**

In `internal/pages/model.go`, update both structs:

```go
package pages

type TreeNode struct {
	Name      string     `json:"name"`
	Path      string     `json:"path,omitempty"`
	Title     string     `json:"title,omitempty"`
	IsDir     bool       `json:"is_dir"`
	Children  []TreeNode `json:"children,omitempty"`
	ShowDate  bool       `json:"show_date,omitempty"`
	CreatedAt string     `json:"created_at,omitempty"`
	Published bool       `json:"published"`
}

type PageResponse struct {
	Path        string `json:"path"`
	Title       string `json:"title"`
	Content     string `json:"content"`
	Description string `json:"description"`
	ViewCount   int    `json:"view_count"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
	ShowDate    bool   `json:"show_date"`
	Published   bool   `json:"published"`
}

type PageRequest struct {
	Title     string  `json:"title"`
	Content   string  `json:"content"`
	ShowDate  *bool   `json:"show_date,omitempty"`
	Published *bool   `json:"published,omitempty"`
	CreatedAt *string `json:"created_at,omitempty"`
	Manual    *bool   `json:"manual,omitempty"`
}
```

- [ ] **Step 3: Update `GetPage` to select the description column**

In `internal/pages/service.go`, the `GetPage` method currently runs:

```go
err = s.db.QueryRow(
    "SELECT path, title, view_count, created_at, updated_at, show_date, published FROM pages WHERE path = ?",
    clean,
).Scan(&resp.Path, &resp.Title, &resp.ViewCount, &resp.CreatedAt, &resp.UpdatedAt, &resp.ShowDate, &resp.Published)
```

Change to:

```go
err = s.db.QueryRow(
    "SELECT path, title, description, view_count, created_at, updated_at, show_date, published FROM pages WHERE path = ?",
    clean,
).Scan(&resp.Path, &resp.Title, &resp.Description, &resp.ViewCount, &resp.CreatedAt, &resp.UpdatedAt, &resp.ShowDate, &resp.Published)
```

The self-heal branch below (`sql.ErrNoRows` case) does not need changes — the default value `''` is fine for a freshly-inserted row.

- [ ] **Step 4: Update the test `setupTestDB` helper**

In `internal/pages/service_test.go`, update the `CREATE TABLE` in `setupTestDB` to include the new column:

```go
db.Exec(`CREATE TABLE pages (
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
```

- [ ] **Step 5: Apply migration & run tests**

```bash
go test ./...
```

This also exercises the migration runner indirectly through existing tests that open real databases. Expected: all tests pass.

- [ ] **Step 6: Commit**

```bash
git add migrations/007_add_description.up.sql migrations/007_add_description.down.sql internal/pages/model.go internal/pages/service.go internal/pages/service_test.go
git commit -m "feat(pages): add description column and field"
```

---

## Task 4: `contentSnippet` fallback (pure function)

**Files:**
- Create: `internal/pages/description.go`
- Create: `internal/pages/description_test.go`

- [ ] **Step 1: Write the failing tests**

`internal/pages/description_test.go`:

```go
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
```

- [ ] **Step 2: Run tests to verify failure**

```bash
go test ./internal/pages/ -run TestContentSnippet -v
```
Expected: FAIL — `contentSnippet` undefined.

- [ ] **Step 3: Implement `contentSnippet`**

Create `internal/pages/description.go`:

```go
package pages

import (
	"regexp"
	"strings"
)

var (
	reFencedCode = regexp.MustCompile("(?s)```.*?```")
	reInlineCode = regexp.MustCompile("`[^`]*`")
	reImage      = regexp.MustCompile(`!\[([^\]]*)\]\([^)]*\)`)
	reLink       = regexp.MustCompile(`\[([^\]]+)\]\([^)]*\)`)
	reEmphasis   = regexp.MustCompile(`[*_~]+`)
	reWhitespace = regexp.MustCompile(`\s+`)
)

// contentSnippet strips markdown syntax from content and returns the first
// ≤160 chars at a word boundary. Always safe and fast, never returns error.
func contentSnippet(content string) string {
	s := content

	// Remove fenced code blocks entirely
	s = reFencedCode.ReplaceAllString(s, " ")
	// Replace images with alt text
	s = reImage.ReplaceAllString(s, "$1")
	// Replace links with their text
	s = reLink.ReplaceAllString(s, "$1")
	// Remove inline code backticks (keep contents)
	s = reInlineCode.ReplaceAllStringFunc(s, func(m string) string {
		return strings.Trim(m, "`")
	})
	// Strip emphasis markers
	s = reEmphasis.ReplaceAllString(s, "")

	// Strip leading line markers (#, >, -, *, numbered) at start of each line
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		trimmed := strings.TrimLeft(line, " \t")
		// Headings
		trimmed = strings.TrimLeft(trimmed, "#")
		// Blockquotes
		trimmed = strings.TrimLeft(trimmed, ">")
		// List bullets
		if len(trimmed) > 0 && (trimmed[0] == '-' || trimmed[0] == '*' || trimmed[0] == '+') {
			if len(trimmed) > 1 && trimmed[1] == ' ' {
				trimmed = trimmed[2:]
			}
		}
		// Numbered list (1. etc.)
		for len(trimmed) > 0 && trimmed[0] >= '0' && trimmed[0] <= '9' {
			trimmed = trimmed[1:]
		}
		trimmed = strings.TrimLeft(trimmed, ".)")
		trimmed = strings.TrimLeft(trimmed, " ")
		lines[i] = trimmed
	}
	s = strings.Join(lines, " ")

	// Collapse whitespace
	s = reWhitespace.ReplaceAllString(s, " ")
	s = strings.TrimSpace(s)

	// Cap at 160 at word boundary
	if len(s) <= 160 {
		return s
	}
	cut := s[:160]
	if idx := strings.LastIndex(cut, " "); idx > 0 {
		return cut[:idx]
	}
	return cut
}
```

- [ ] **Step 4: Run tests to verify pass**

```bash
go test ./internal/pages/ -run TestContentSnippet -v
```
Expected: all 6 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/pages/description.go internal/pages/description_test.go
git commit -m "feat(pages): add contentSnippet fallback for descriptions"
```

---

## Task 5: Description `Generator` with Claude client seam

**Files:**
- Modify: `internal/pages/description.go`
- Modify: `internal/pages/description_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `internal/pages/description_test.go`:

```go
import (
	"context"
	"errors"
	// ... keep existing imports
)

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
```

`setupTestDB` creates a `pages` table but does not create a `settings` table — extend it to create one. Edit `internal/pages/service_test.go`'s `setupTestDB` helper to also run:

```go
db.Exec(`CREATE TABLE settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL
)`)
```

- [ ] **Step 2: Run tests to verify failure**

```bash
go test ./internal/pages/ -run TestGenerator -v
```
Expected: FAIL — `Generator`, `ClaudeRequest`, `ClaudeClient` undefined.

- [ ] **Step 3: Implement the generator**

Append to `internal/pages/description.go`:

```go
import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

const descriptionModel = "claude-haiku-4-5-20251001"

const descriptionSystemPrompt = "Write a meta description for a webpage. Output a single sentence, 130-160 characters, no quotes, no trailing punctuation other than a period. Describe what the reader will learn or get from the page, not meta-commentary about the page itself."

// ClaudeRequest is the minimal subset of Anthropic's /v1/messages body we use.
type ClaudeRequest struct {
	Model     string        `json:"model"`
	MaxTokens int           `json:"max_tokens"`
	System    string        `json:"system,omitempty"`
	Messages  []ClaudeMsg   `json:"messages"`
	Temperature float64     `json:"temperature,omitempty"`
}

type ClaudeMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ClaudeClient sends a message to the Anthropic API and returns the response text.
type ClaudeClient interface {
	CreateMessage(ctx context.Context, apiKey string, req ClaudeRequest) (string, error)
}

// httpClaudeClient is the production implementation.
type httpClaudeClient struct {
	http *http.Client
}

func (c *httpClaudeClient) CreateMessage(ctx context.Context, apiKey string, req ClaudeRequest) (string, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return "", err
	}
	hr, err := http.NewRequestWithContext(ctx, "POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	hr.Header.Set("Content-Type", "application/json")
	hr.Header.Set("x-api-key", apiKey)
	hr.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.http.Do(hr)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("anthropic API %d: %s", resp.StatusCode, string(data))
	}
	var parsed struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return "", err
	}
	for _, c := range parsed.Content {
		if c.Type == "text" && c.Text != "" {
			return c.Text, nil
		}
	}
	return "", fmt.Errorf("no text content in response")
}

// Generator produces meta descriptions via Claude, with a content-derived fallback.
type Generator struct {
	db      *sql.DB
	client  ClaudeClient
	timeout time.Duration
}

// NewGenerator returns a Generator that uses the default HTTP Claude client.
func NewGenerator(db *sql.DB, timeout time.Duration) *Generator {
	return &Generator{
		db:      db,
		client:  &httpClaudeClient{http: &http.Client{Timeout: timeout}},
		timeout: timeout,
	}
}

// Generate returns a description for the given page content. Always non-empty:
// AI result, or the content-derived fallback on any failure.
func (g *Generator) Generate(ctx context.Context, title, content string) string {
	apiKey := g.loadAPIKey()
	if apiKey == "" {
		return contentSnippet(content)
	}

	userMsg := "Title: " + title + "\n\nContent:\n" + truncate(content, 4000)
	req := ClaudeRequest{
		Model:       descriptionModel,
		MaxTokens:   120,
		Temperature: 0.3,
		System:      descriptionSystemPrompt,
		Messages:    []ClaudeMsg{{Role: "user", Content: userMsg}},
	}

	callCtx, cancel := context.WithTimeout(ctx, g.timeout)
	defer cancel()

	text, err := g.client.CreateMessage(callCtx, apiKey, req)
	if err != nil {
		log.Printf("description: AI generation failed: %v", err)
		return contentSnippet(content)
	}
	return postprocess(text, content)
}

func (g *Generator) loadAPIKey() string {
	var key string
	row := g.db.QueryRow(`SELECT value FROM settings WHERE key = 'ai_api_key'`)
	if err := row.Scan(&key); err != nil {
		return ""
	}
	return key
}

func postprocess(text, content string) string {
	s := strings.TrimSpace(text)
	// Strip surrounding quotes (ASCII + curly)
	for _, q := range []string{`"`, `'`, "“", "”", "‘", "’"} {
		s = strings.TrimPrefix(s, q)
		s = strings.TrimSuffix(s, q)
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return contentSnippet(content)
	}
	// Cap at 160 at word boundary
	if len(s) <= 160 {
		return s
	}
	cut := s[:160]
	if idx := strings.LastIndex(cut, " "); idx > 0 {
		return cut[:idx]
	}
	return cut
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}
```

- [ ] **Step 4: Run tests to verify pass**

```bash
go test ./internal/pages/ -run "TestGenerator|TestContentSnippet" -v
```
Expected: all PASS.

- [ ] **Step 5: Run full pages package test suite**

```bash
go test ./internal/pages/ -v
```
Expected: all existing tests still pass.

- [ ] **Step 6: Commit**

```bash
git add internal/pages/description.go internal/pages/description_test.go internal/pages/service_test.go
git commit -m "feat(pages): add description Generator with Claude Haiku + fallback"
```

---

## Task 6: Wire `Generator` into `UpdatePage` (manual save only)

**Files:**
- Modify: `internal/pages/handler.go`
- Modify: `internal/pages/description_test.go` (integration-style test)
- Modify: `cmd/server/main.go` (inject Generator)

- [ ] **Step 1: Write the failing tests**

Append to `internal/pages/description_test.go`:

```go
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
```

Ensure these imports are at the top of `description_test.go`:

```go
import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)
```

- [ ] **Step 2: Run tests to verify failure**

```bash
go test ./internal/pages/ -run TestHandlerUpdatePage -v
```
Expected: FAIL — `descGen` field missing on Handler.

- [ ] **Step 3: Add `descGen` to the Handler and wire it in `UpdatePage`**

In `internal/pages/handler.go`, update the Handler struct and constructor:

```go
type Handler struct {
	svc     *Service
	baseURL string
	descGen *Generator
}

func NewHandler(svc *Service, baseURL string, descGen *Generator) *Handler {
	return &Handler{svc: svc, baseURL: baseURL, descGen: descGen}
}
```

Modify `UpdatePage` — after the `UpdatePage` call succeeds (after `httputil.JSONError` checks but before the `json.NewEncoder` response), add description regeneration on manual save:

```go
func (h *Handler) UpdatePage(w http.ResponseWriter, r *http.Request) {
	pagePath := extractPath(r)

	var req PageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.JSONError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Title == "" {
		httputil.JSONError(w, "title is required", http.StatusBadRequest)
		return
	}

	err := h.svc.UpdatePage(pagePath, req.Title, req.Content, req.ShowDate, req.Published, req.CreatedAt)
	if err == ErrNotFound {
		httputil.JSONError(w, "page not found", http.StatusNotFound)
		return
	}
	if err == ErrInvalidPath {
		httputil.JSONError(w, "invalid path", http.StatusBadRequest)
		return
	}
	if err != nil {
		httputil.JSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	// On manual save, regenerate description (synchronous, best-effort).
	if req.Manual != nil && *req.Manual && h.descGen != nil {
		desc := h.descGen.Generate(r.Context(), req.Title, req.Content)
		if _, dbErr := h.svc.db.Exec("UPDATE pages SET description = ? WHERE path = ?", desc, pagePath); dbErr != nil {
			log.Printf("description: write failed for %s: %v", pagePath, dbErr)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"path": pagePath, "title": req.Title})
}
```

Add `"log"` to the imports at the top of `handler.go`.

- [ ] **Step 4: Update `main.go` to construct the Generator**

In `cmd/server/main.go`, after the `pagesSvc := pages.NewService(...)` line, add:

```go
descGen := pages.NewGenerator(db, 10*time.Second)
pagesHandler := pages.NewHandler(pagesSvc, cfg.BaseURL, descGen)
```

Replace the previous `pagesHandler := pages.NewHandler(pagesSvc, cfg.BaseURL)` line. Ensure `"time"` is imported (it already is per existing code).

- [ ] **Step 5: Run tests**

```bash
go test ./...
```
Expected: all PASS including the two new handler tests.

- [ ] **Step 6: Commit**

```bash
git add internal/pages/handler.go internal/pages/description_test.go cmd/server/main.go
git commit -m "feat(pages): regenerate description on manual save"
```

---

## Task 7: Backfill empty descriptions on startup

**Files:**
- Modify: `internal/pages/description.go`
- Modify: `internal/pages/description_test.go`
- Modify: `cmd/server/main.go`

- [ ] **Step 1: Write the failing tests**

Append to `internal/pages/description_test.go`:

```go
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
```

Add `"fmt"` to the imports of `description_test.go`.

- [ ] **Step 2: Run tests to verify failure**

```bash
go test ./internal/pages/ -run TestBackfill -v
```
Expected: FAIL — `BackfillEmpty` undefined.

- [ ] **Step 3: Implement `BackfillEmpty`**

Append to `internal/pages/description.go`:

```go
// BackfillEmpty fills empty descriptions one at a time. Intended to run once
// on server startup in a background goroutine. Returns when no more empty
// descriptions exist or ctx is canceled.
func (g *Generator) BackfillEmpty(ctx context.Context, svc *Service) {
	const delay = 500 * time.Millisecond

	for {
		if ctx.Err() != nil {
			return
		}

		var path string
		err := g.db.QueryRowContext(ctx, `
			SELECT path FROM pages
			WHERE description = '' AND published = 1
			ORDER BY updated_at DESC
			LIMIT 1
		`).Scan(&path)
		if err == sql.ErrNoRows {
			return
		}
		if err != nil {
			log.Printf("backfill: query failed: %v", err)
			return
		}

		page, err := svc.GetPage(path)
		if err != nil {
			log.Printf("backfill: load %s failed: %v", path, err)
			// Mark with a space so we don't retry forever on a single broken page.
			g.db.Exec(`UPDATE pages SET description = ' ' WHERE path = ?`, path)
			continue
		}

		desc := g.Generate(ctx, page.Title, page.Content)
		if _, err := g.db.ExecContext(ctx, `UPDATE pages SET description = ? WHERE path = ?`, desc, path); err != nil {
			log.Printf("backfill: write %s failed: %v", path, err)
			return
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}
	}
}
```

- [ ] **Step 4: Run tests to verify pass**

```bash
go test ./internal/pages/ -run TestBackfill -v
```
Expected: PASS. (Note: `TestBackfillEmptyPopulatesDescriptions` has a 500ms delay between calls — it only backfills one page, so runs in <1s.)

- [ ] **Step 5: Wire backfill into server startup**

In `cmd/server/main.go`, after `descGen := pages.NewGenerator(...)` and before the server starts listening, add:

```go
backfillCtx, backfillCancel := context.WithCancel(context.Background())
go descGen.BackfillEmpty(backfillCtx, pagesSvc)
```

At shutdown (after `srv.Shutdown(ctx)` completes), add:

```go
backfillCancel()
```

- [ ] **Step 6: Build & run full tests**

```bash
go build ./...
go test ./...
```
Expected: all pass.

- [ ] **Step 7: Commit**

```bash
git add internal/pages/description.go internal/pages/description_test.go cmd/server/main.go
git commit -m "feat(pages): backfill empty descriptions on startup"
```

---

## Task 8: Sitemap builder

**Files:**
- Create: `internal/pages/sitemap.go`
- Create: `internal/pages/sitemap_test.go`

- [ ] **Step 1: Write the failing tests**

`internal/pages/sitemap_test.go`:

```go
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
```

- [ ] **Step 2: Run tests to verify failure**

```bash
go test ./internal/pages/ -run TestBuildSitemap -v
```
Expected: FAIL — `BuildSitemap` undefined.

- [ ] **Step 3: Implement BuildSitemap**

Create `internal/pages/sitemap.go`:

```go
package pages

import (
	"database/sql"
	"encoding/xml"
	"fmt"
	"time"
)

type sitemapURL struct {
	XMLName xml.Name `xml:"url"`
	Loc     string   `xml:"loc"`
	LastMod string   `xml:"lastmod,omitempty"`
}

type sitemapURLSet struct {
	XMLName xml.Name     `xml:"urlset"`
	XMLNS   string       `xml:"xmlns,attr"`
	URLs    []sitemapURL `xml:"url"`
}

// BuildSitemap returns the full sitemap.xml body for all published pages.
// The `home` path renders as a bare baseURL (no /home suffix).
func BuildSitemap(db *sql.DB, baseURL string) ([]byte, error) {
	rows, err := db.Query(`SELECT path, updated_at FROM pages WHERE published = 1 ORDER BY updated_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("query pages: %w", err)
	}
	defer rows.Close()

	var urls []sitemapURL
	for rows.Next() {
		var path, updatedAt string
		if err := rows.Scan(&path, &updatedAt); err != nil {
			return nil, err
		}
		t, err := time.Parse(time.RFC3339, updatedAt)
		if err != nil {
			t = time.Now()
		}
		loc := baseURL + "/" + path
		if path == "home" {
			loc = baseURL
		}
		urls = append(urls, sitemapURL{
			Loc:     loc,
			LastMod: t.Format("2006-01-02"),
		})
	}

	set := sitemapURLSet{
		XMLNS: "http://www.sitemaps.org/schemas/sitemap/0.9",
		URLs:  urls,
	}

	out, err := xml.MarshalIndent(set, "", "  ")
	if err != nil {
		return nil, err
	}
	return append([]byte(xml.Header), out...), nil
}
```

- [ ] **Step 4: Run tests to verify pass**

```bash
go test ./internal/pages/ -run TestBuildSitemap -v
```
Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/pages/sitemap.go internal/pages/sitemap_test.go
git commit -m "feat(pages): add sitemap builder"
```

---

## Task 9: Sitemap HTTP handler + route

**Files:**
- Modify: `internal/pages/handler.go`
- Modify: `cmd/server/main.go`

- [ ] **Step 1: Add the handler method**

In `internal/pages/handler.go`, append a new method alongside `GetRSS`:

```go
func (h *Handler) GetSitemap(w http.ResponseWriter, r *http.Request) {
	xml, err := BuildSitemap(h.svc.db, h.baseURL)
	if err != nil {
		http.Error(w, "failed to build sitemap", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Write(xml)
}
```

- [ ] **Step 2: Register the route**

In `cmd/server/main.go`, find the `mux.HandleFunc("GET /feed.xml", ...)` line and add just after it:

```go
mux.HandleFunc("GET /sitemap.xml", pagesHandler.GetSitemap)
```

- [ ] **Step 3: Build and run**

```bash
go build ./...
go test ./...
```
Expected: clean build, all tests pass.

- [ ] **Step 4: Manual smoke test (optional)**

Start the server (`make build-run` or equivalent) and:

```bash
curl -s http://localhost:8080/sitemap.xml | head -15
```

Expect `<urlset>` with `<url>` entries matching DB contents.

- [ ] **Step 5: Commit**

```bash
git add internal/pages/handler.go cmd/server/main.go
git commit -m "feat(server): serve sitemap.xml"
```

---

## Task 10: robots.txt handler

**Files:**
- Modify: `cmd/server/main.go`

- [ ] **Step 1: Register inline handler**

In `cmd/server/main.go`, after the `/sitemap.xml` route, add:

```go
mux.HandleFunc("GET /robots.txt", func(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "text/plain; charset=utf-8")
    w.Header().Set("Cache-Control", "public, max-age=3600")
    fmt.Fprintf(w, "User-agent: *\nDisallow: /admin/\nDisallow: /api/\nAllow: /\n\nSitemap: %s/sitemap.xml\n", cfg.BaseURL)
})
```

Add `"fmt"` to the imports at the top of `main.go` if not present.

- [ ] **Step 2: Build**

```bash
go build ./...
```
Expected: clean build.

- [ ] **Step 3: Manual smoke test (optional)**

```bash
curl -s http://localhost:8080/robots.txt
```

Expected output:

```
User-agent: *
Disallow: /admin/
Disallow: /api/
Allow: /

Sitemap: https://mees.space/sitemap.xml
```

- [ ] **Step 4: Commit**

```bash
git add cmd/server/main.go
git commit -m "feat(server): serve robots.txt"
```

---

## Task 11: HTML meta injector

**Files:**
- Create: `internal/seo/inject.go`
- Create: `internal/seo/inject_test.go`

- [ ] **Step 1: Write the failing tests**

`internal/seo/inject_test.go`:

```go
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
```

- [ ] **Step 2: Run tests to verify failure**

```bash
go test ./internal/seo/ -v
```
Expected: compile errors — package doesn't exist.

- [ ] **Step 3: Implement the injector**

Create `internal/seo/inject.go`:

```go
package seo

import (
	"bytes"
	"fmt"
	"html"
	"os"
	"path/filepath"
)

// PageMeta is the per-request data needed to customize the HTML head.
type PageMeta struct {
	Title        string
	Description  string
	CanonicalURL string
	OGImage      string
	NoIndex      bool
}

// Injector holds the index.html template and produces customized HTML.
type Injector struct {
	template []byte
}

// NewInjector reads dist/index.html and verifies the </head> anchor exists.
// Returns an error if the file is missing or the anchor can't be found.
func NewInjector(distDir string) (*Injector, error) {
	path := filepath.Join(distDir, "index.html")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	if !bytes.Contains(data, []byte("</head>")) {
		return nil, fmt.Errorf("%s: missing </head> anchor; cannot inject SEO metadata", path)
	}
	return &Injector{template: data}, nil
}

// Inject returns a customized copy of the template with per-page <head> tags
// inserted just before the first </head>. Safe for concurrent use.
func (i *Injector) Inject(m PageMeta) []byte {
	fragment := buildFragment(m)
	replacement := append(fragment, []byte("</head>")...)
	return bytes.Replace(i.template, []byte("</head>"), replacement, 1)
}

// Raw returns the unmodified template.
func (i *Injector) Raw() []byte {
	return i.template
}

func buildFragment(m PageMeta) []byte {
	title := html.EscapeString(m.Title)
	desc := html.EscapeString(m.Description)
	canon := html.EscapeString(m.CanonicalURL)
	img := html.EscapeString(m.OGImage)

	var b bytes.Buffer
	fmt.Fprintf(&b, `<title>%s</title>`, title)
	fmt.Fprintf(&b, `<meta name="description" content="%s">`, desc)
	if canon != "" {
		fmt.Fprintf(&b, `<link rel="canonical" href="%s">`, canon)
	}
	fmt.Fprintf(&b, `<meta property="og:title" content="%s">`, title)
	fmt.Fprintf(&b, `<meta property="og:description" content="%s">`, desc)
	if canon != "" {
		fmt.Fprintf(&b, `<meta property="og:url" content="%s">`, canon)
	}
	if img != "" {
		fmt.Fprintf(&b, `<meta property="og:image" content="%s">`, img)
	}
	fmt.Fprintf(&b, `<meta property="og:type" content="article">`)
	fmt.Fprintf(&b, `<meta name="twitter:card" content="summary_large_image">`)
	fmt.Fprintf(&b, `<meta name="twitter:title" content="%s">`, title)
	fmt.Fprintf(&b, `<meta name="twitter:description" content="%s">`, desc)
	if img != "" {
		fmt.Fprintf(&b, `<meta name="twitter:image" content="%s">`, img)
	}
	if m.NoIndex {
		fmt.Fprintf(&b, `<meta name="robots" content="noindex">`)
	}
	return b.Bytes()
}
```

- [ ] **Step 4: Run tests to verify pass**

```bash
go test ./internal/seo/ -v
```
Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/seo/inject.go internal/seo/inject_test.go
git commit -m "feat(seo): add HTML meta injector"
```

---

## Task 12: Wire injector into `staticHandler` + startup

**Files:**
- Modify: `cmd/server/main.go`

- [ ] **Step 1: Update the staticHandler signature and behavior**

In `cmd/server/main.go`, replace the `staticHandler` function entirely:

```go
func staticHandler(distDir, baseURL string, pagesSvc *pages.Service, injector *seo.Injector) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		urlPath := r.URL.Path
		if urlPath == "/" {
			serveContentPage(w, r, "", baseURL, pagesSvc, injector)
			return
		}

		// Try exact file
		filePath := filepath.Join(distDir, filepath.FromSlash(urlPath))
		if serveIfExists(w, r, filePath) {
			return
		}

		// Try .html extension
		if !strings.HasSuffix(urlPath, ".html") {
			htmlPath := filepath.Join(distDir, filepath.FromSlash(urlPath+".html"))
			if serveIfExists(w, r, htmlPath) {
				return
			}
		}

		// Try index.html in subdirectory (e.g., /admin/editor/)
		indexPath := filepath.Join(distDir, filepath.FromSlash(urlPath), "index.html")
		if serveIfExists(w, r, indexPath) {
			return
		}

		// Admin paths fall back to raw shell (don't inject SEO)
		if strings.HasPrefix(urlPath, "/admin") {
			writeHTML(w, injector.Raw())
			return
		}

		// Content page fallback: strip leading slash and attempt lookup.
		pagePath := strings.TrimPrefix(urlPath, "/")
		serveContentPage(w, r, pagePath, baseURL, pagesSvc, injector)
	}
}

func serveContentPage(w http.ResponseWriter, r *http.Request, pagePath, baseURL string, pagesSvc *pages.Service, injector *seo.Injector) {
	if pagePath == "" {
		pagePath = "home"
	}

	page, err := pagesSvc.GetPage(pagePath)
	if err != nil {
		// Not found or invalid — fall back to raw shell for client-side 404
		writeHTML(w, injector.Raw())
		return
	}

	canonical := baseURL + "/" + pagePath
	if pagePath == "home" {
		canonical = baseURL
	}

	meta := seo.PageMeta{
		Title:        page.Title,
		Description:  page.Description,
		CanonicalURL: canonical,
		OGImage:      baseURL + "/mees.png",
		NoIndex:      !page.Published,
	}

	if !page.Published {
		// Don't leak draft details to crawlers
		meta.Title = "Draft — Mees Brinkhuis"
		meta.Description = ""
	}

	if page.Description == "" {
		// No generated description yet (backfill may not have run); leave empty
		// rather than fabricate one.
	}

	writeHTML(w, injector.Inject(meta))
}

func writeHTML(w http.ResponseWriter, body []byte) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(body)
}

func serveIfExists(w http.ResponseWriter, r *http.Request, path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	http.ServeFile(w, r, path)
	return true
}
```

Add `"mees.space/internal/seo"` to the imports if not present.

- [ ] **Step 2: Construct the Injector at startup and pass it to staticHandler**

In `main()`, after the `descGen := ...` line and before the mux is set up, add:

```go
injector, err := seo.NewInjector(cfg.DistDir)
if err != nil {
    log.Fatal("seo injector:", err)
}
```

Then update the catch-all registration from:

```go
mux.HandleFunc("GET /{path...}", staticHandler(cfg.DistDir))
```

to:

```go
mux.HandleFunc("GET /{path...}", staticHandler(cfg.DistDir, cfg.BaseURL, pagesSvc, injector))
```

- [ ] **Step 3: Build and run tests**

```bash
go build ./...
go test ./...
```
Expected: clean build, all tests pass. The frontend must be built (`dist/index.html` must exist with a `</head>`) before the server can start.

- [ ] **Step 4: Manual smoke test**

Ensure frontend is built:

```bash
bash -c 'export NVM_DIR="$HOME/.nvm"; source "$NVM_DIR/nvm.sh"; nvm use 20 > /dev/null; cd frontend && npm run build'
```

Then run the server and check:

```bash
curl -s http://localhost:8080/ | grep -E 'og:|canonical|description' | head -10
```
Expect per-page values for the home page.

Visit a known content path (e.g., a page that exists in content/):
```bash
curl -s http://localhost:8080/some-existing-page | grep -E 'og:|canonical'
```
Expect per-page values referencing that path.

- [ ] **Step 5: Commit**

```bash
git add cmd/server/main.go
git commit -m "feat(server): inject per-page SEO meta into content pages"
```

---

## Task 13: Frontend editor — Save button label + `manual: true` flag

**Files:**
- Modify: `frontend/src/app/admin/editor/page.tsx`

- [ ] **Step 1: Find the manual `savePage` function**

Around line 109 in `frontend/src/app/admin/editor/page.tsx`, the `savePage` function sends a PUT. Modify its body to include `manual: true`. The exact current code (line 109 area):

Find the block that looks like:

```tsx
const savePage = async () => {
    setSaving(true);
    const res = await apiFetch(`/api/pages/${selectedPath}`, {
      method: "PUT",
      body: JSON.stringify({
        title,
        content,
        show_date: showDate,
        published,
        created_at: createdAt,
      }),
    });
    setSaving(false);
    setMessage(res.ok ? "Saved" : "Save failed");
};
```

And replace with (adding `manual: true`):

```tsx
const savePage = async () => {
    setSaving(true);
    const res = await apiFetch(`/api/pages/${selectedPath}`, {
      method: "PUT",
      body: JSON.stringify({
        title,
        content,
        show_date: showDate,
        published,
        created_at: createdAt,
        manual: true,
      }),
    });
    setSaving(false);
    setMessage(res.ok ? "Saved" : "Save failed");
};
```

IMPORTANT: do NOT modify the auto-save body around line 68 — it must continue to omit `manual`.

- [ ] **Step 2: Update the Save button label**

Find the Save button around line 1000–1016:

```tsx
<button
  onClick={savePage}
  ...
  {saving ? "Saving..." : "Save"}
</button>
```

Change the label expression to:

```tsx
{saving ? "Saving & describing…" : "Save & regenerate description"}
```

- [ ] **Step 3: Verify the build**

```bash
bash -c '
export NVM_DIR="$HOME/.nvm"
source "$NVM_DIR/nvm.sh"
nvm use 20 > /dev/null
cd /home/mees/git/mees.space/frontend
npm run build
'
```
Expected: build succeeds.

- [ ] **Step 4: Manual smoke test (optional; subagents skip this)**

Start `npm run dev` (or the full stack), log into `/admin/editor`, open a page, click Save. Confirm:
- Button text reads "Saving & describing…" while awaiting.
- Save completes (may take a few seconds with API key set, ~1s without).
- Description column in DB is populated (`sqlite3 mees.db 'SELECT path, description FROM pages'`).

- [ ] **Step 5: Commit**

```bash
git add frontend/src/app/admin/editor/page.tsx
git commit -m "feat(frontend): Save button triggers description regeneration"
```

---

## Done Criteria

All 13 tasks committed. `go test ./...` passes. `cd frontend && npm run build` succeeds. Manual browser spot checks from the spec's Verification Plan ideally run, or noted as remaining for the controller.
