# Editable SEO Description Prompt — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the SEO meta-description system prompt editable from `/admin/settings`, mirroring the existing `ai_system_prompt` pattern. Empty user value falls back to a hardcoded Go default.

**Architecture:** Add one settings key (`ai_description_prompt`). `internal/pages` exports the default prompt and reads the user override from the `settings` table at generation time. `internal/settings` exposes both the user value and the default string in `GET /api/settings`. The settings UI adds one textarea with the default shown as placeholder.

**Tech Stack:** Go 1.26 (backend), SQLite (settings table), Next.js 15 / React 19 / TypeScript (frontend).

**Spec:** `docs/superpowers/specs/2026-04-24-editable-seo-prompt-design.md`

---

## File Structure

**Modify:**
- `internal/pages/description.go` — rename const to exported, add `loadDescriptionPrompt()`, use fallback in `Generate()`.
- `internal/pages/description_test.go` — extend `stubClient` to capture `req.System`; add tests for the fallback logic.
- `internal/settings/handler.go` — add `AIDescriptionPrompt` request/response fields and `AIDescriptionPromptDefault` response field.
- `internal/settings/handler_test.go` — round-trip test + default-exposure test.
- `frontend/src/app/admin/settings/page.tsx` — add a new textarea section for the description prompt.

**Create:** None.

---

## Task 1: Export the default description prompt constant

**Files:**
- Modify: `internal/pages/description.go:87`

A trivial refactor so the settings handler can reference the default string without duplicating it. No behavior change, no new test needed — existing tests continue to pass.

- [ ] **Step 1: Run the existing tests to confirm the baseline is green**

Run: `go test ./internal/pages/... ./internal/settings/...`
Expected: PASS (all existing tests green).

- [ ] **Step 2: Rename the constant**

Edit `internal/pages/description.go`. Find:

```go
const descriptionSystemPrompt = "Write a meta description for a webpage. Output a single sentence, 130-160 characters, no quotes, no trailing punctuation other than a period. Describe what the reader will learn or get from the page, not meta-commentary about the page itself."
```

Replace with:

```go
const DefaultDescriptionPrompt = "Write a meta description for a webpage. Output a single sentence, 130-160 characters, no quotes, no trailing punctuation other than a period. Describe what the reader will learn or get from the page, not meta-commentary about the page itself."
```

Then find the single usage inside `Generate()`:

```go
		System:      descriptionSystemPrompt,
```

Replace with:

```go
		System:      DefaultDescriptionPrompt,
```

- [ ] **Step 3: Run tests to verify nothing broke**

Run: `go test ./internal/pages/...`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/pages/description.go
git commit -m "refactor(pages): export DefaultDescriptionPrompt constant"
```

---

## Task 2: Make `stubClient` capture the outgoing System prompt

**Files:**
- Modify: `internal/pages/description_test.go:99-106`

The existing `stubClient` discards the request. To assert that the right prompt is being passed to the AI client, we add a capture field. This is a pure test-infra enhancement with no production impact.

- [ ] **Step 1: Update stubClient to capture System**

Edit `internal/pages/description_test.go`. Find:

```go
type stubClient struct {
	response string
	err      error
}

func (s *stubClient) CreateMessage(ctx context.Context, apiKey string, req ClaudeRequest) (string, error) {
	return s.response, s.err
}
```

Replace with:

```go
type stubClient struct {
	response   string
	err        error
	lastSystem string // captured from most recent call
}

func (s *stubClient) CreateMessage(ctx context.Context, apiKey string, req ClaudeRequest) (string, error) {
	s.lastSystem = req.System
	return s.response, s.err
}
```

- [ ] **Step 2: Run tests to verify nothing broke**

Run: `go test ./internal/pages/...`
Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add internal/pages/description_test.go
git commit -m "test(pages): capture System prompt in stubClient"
```

---

## Task 3: Test and implement DB-backed custom description prompt

**Files:**
- Modify: `internal/pages/description_test.go` (append new tests at end)
- Modify: `internal/pages/description.go` (add helper + wire fallback in `Generate()`)

This is the core behavior change. TDD cycle: write a failing test that seeds the setting and asserts the custom prompt reaches the client, then implement the helper and fallback logic to make it pass.

- [ ] **Step 1: Add failing test for custom prompt**

Append to `internal/pages/description_test.go`:

```go
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
```

- [ ] **Step 2: Run the new test and verify it fails**

Run: `go test ./internal/pages/... -run TestGeneratorUsesCustomDescriptionPromptFromDB -v`
Expected: FAIL. `stub.lastSystem` will equal the default prompt, not `"Custom prompt here."`, because `Generate()` still hardcodes `DefaultDescriptionPrompt`.

- [ ] **Step 3: Add the helper and wire the fallback**

Edit `internal/pages/description.go`. Find:

```go
func (g *Generator) loadAPIKey() string {
	var key string
	row := g.db.QueryRow(`SELECT value FROM settings WHERE key = 'ai_api_key'`)
	if err := row.Scan(&key); err != nil {
		return ""
	}
	return key
}
```

Add this new method immediately after `loadAPIKey`:

```go
func (g *Generator) loadDescriptionPrompt() string {
	var prompt string
	row := g.db.QueryRow(`SELECT value FROM settings WHERE key = 'ai_description_prompt'`)
	if err := row.Scan(&prompt); err != nil {
		return ""
	}
	return strings.TrimSpace(prompt)
}
```

Then find the `Generate` method body, specifically:

```go
	userMsg := "Title: " + title + "\n\nContent:\n" + truncate(content, 4000)
	req := ClaudeRequest{
		Model:       descriptionModel,
		MaxTokens:   120,
		Temperature: 0.3,
		System:      DefaultDescriptionPrompt,
		Messages:    []ClaudeMsg{{Role: "user", Content: userMsg}},
	}
```

Replace with:

```go
	userMsg := "Title: " + title + "\n\nContent:\n" + truncate(content, 4000)
	system := g.loadDescriptionPrompt()
	if system == "" {
		system = DefaultDescriptionPrompt
	}
	req := ClaudeRequest{
		Model:       descriptionModel,
		MaxTokens:   120,
		Temperature: 0.3,
		System:      system,
		Messages:    []ClaudeMsg{{Role: "user", Content: userMsg}},
	}
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `go test ./internal/pages/... -run TestGeneratorUsesCustomDescriptionPromptFromDB -v`
Expected: PASS.

- [ ] **Step 5: Run the full package tests to check for regressions**

Run: `go test ./internal/pages/...`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/pages/description.go internal/pages/description_test.go
git commit -m "feat(pages): load description prompt from settings with default fallback"
```

---

## Task 4: Test the empty-setting fallback path

**Files:**
- Modify: `internal/pages/description_test.go` (append)

Guard against regressions in the "setting is empty or whitespace" branch. These assertions are quick and cheap and cement the contract the UI depends on.

- [ ] **Step 1: Add tests for default fallback**

Append to `internal/pages/description_test.go`:

```go
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
```

- [ ] **Step 2: Run the new tests**

Run: `go test ./internal/pages/... -run 'TestGeneratorUsesDefault' -v`
Expected: PASS for both (the implementation from Task 3 already handles these cases).

- [ ] **Step 3: Commit**

```bash
git add internal/pages/description_test.go
git commit -m "test(pages): assert default description prompt fallback"
```

---

## Task 5: Settings handler — round-trip the new field

**Files:**
- Modify: `internal/settings/handler.go`
- Modify: `internal/settings/handler_test.go`

Add `AIDescriptionPrompt` (both request and response) to the settings API. Uses the same upsert/load pattern as the existing three fields.

- [ ] **Step 1: Write a failing round-trip test**

Edit `internal/settings/handler_test.go`. Find the existing `TestUpdateAndGet` function and replace it with:

```go
func TestUpdateAndGet(t *testing.T) {
	db := setupTestDB(t)
	h := NewHandler(db)

	// Update settings
	prompt := "You are a helpful assistant"
	descPrompt := "Write a 100-char tagline."
	key := "sk-test-1234567890abcdef"
	model := "claude-haiku-4-5"
	body, _ := json.Marshal(SettingsRequest{
		AISystemPrompt:      &prompt,
		AIDescriptionPrompt: &descPrompt,
		AIAPIKey:            &key,
		AIModel:             &model,
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest("PUT", "/api/settings", bytes.NewReader(body))
	h.Update(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("update status = %d, want %d", rr.Code, http.StatusOK)
	}

	// Get settings back
	rr = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/api/settings", nil)
	h.Get(rr, req)

	var resp SettingsResponse
	json.NewDecoder(rr.Body).Decode(&resp)

	if resp.AISystemPrompt != prompt {
		t.Errorf("system prompt = %q, want %q", resp.AISystemPrompt, prompt)
	}
	if resp.AIDescriptionPrompt != descPrompt {
		t.Errorf("description prompt = %q, want %q", resp.AIDescriptionPrompt, descPrompt)
	}
	if resp.AIModel != model {
		t.Errorf("model = %q, want %q", resp.AIModel, model)
	}
	// API key should be masked
	if resp.AIAPIKey == key {
		t.Error("API key should be masked, got raw value")
	}
	if len(resp.AIAPIKey) == 0 {
		t.Error("masked API key should not be empty")
	}
}
```

- [ ] **Step 2: Run the test and verify it fails**

Run: `go test ./internal/settings/... -run TestUpdateAndGet -v`
Expected: FAIL to compile, because `AIDescriptionPrompt` does not exist on `SettingsRequest`/`SettingsResponse`.

- [ ] **Step 3: Add the field and wire through Get/Update**

Edit `internal/settings/handler.go`. Find:

```go
type SettingsResponse struct {
	AISystemPrompt string `json:"ai_system_prompt"`
	AIAPIKey       string `json:"ai_api_key"`
	AIModel        string `json:"ai_model"`
}

type SettingsRequest struct {
	AISystemPrompt *string `json:"ai_system_prompt,omitempty"`
	AIAPIKey       *string `json:"ai_api_key,omitempty"`
	AIModel        *string `json:"ai_model,omitempty"`
}
```

Replace with:

```go
type SettingsResponse struct {
	AISystemPrompt             string `json:"ai_system_prompt"`
	AIDescriptionPrompt        string `json:"ai_description_prompt"`
	AIDescriptionPromptDefault string `json:"ai_description_prompt_default"`
	AIAPIKey                   string `json:"ai_api_key"`
	AIModel                    string `json:"ai_model"`
}

type SettingsRequest struct {
	AISystemPrompt      *string `json:"ai_system_prompt,omitempty"`
	AIDescriptionPrompt *string `json:"ai_description_prompt,omitempty"`
	AIAPIKey            *string `json:"ai_api_key,omitempty"`
	AIModel             *string `json:"ai_model,omitempty"`
}
```

In the same file, find the `Get` method body starting at:

```go
	resp := SettingsResponse{}
	settings := map[string]*string{
		"ai_system_prompt": &resp.AISystemPrompt,
		"ai_api_key":       &resp.AIAPIKey,
		"ai_model":         &resp.AIModel,
	}

	rows, err := h.db.Query("SELECT key, value FROM settings WHERE key IN ('ai_system_prompt', 'ai_api_key', 'ai_model')")
```

Replace with:

```go
	resp := SettingsResponse{}
	settings := map[string]*string{
		"ai_system_prompt":      &resp.AISystemPrompt,
		"ai_description_prompt": &resp.AIDescriptionPrompt,
		"ai_api_key":            &resp.AIAPIKey,
		"ai_model":              &resp.AIModel,
	}

	rows, err := h.db.Query("SELECT key, value FROM settings WHERE key IN ('ai_system_prompt', 'ai_description_prompt', 'ai_api_key', 'ai_model')")
```

Then find the `Update` method, specifically:

```go
	if req.AISystemPrompt != nil {
		if err := upsert("ai_system_prompt", *req.AISystemPrompt); err != nil {
			httputil.JSONError(w, "failed to update settings", http.StatusInternalServerError)
			return
		}
	}

	if req.AIAPIKey != nil {
```

Replace with:

```go
	if req.AISystemPrompt != nil {
		if err := upsert("ai_system_prompt", *req.AISystemPrompt); err != nil {
			httputil.JSONError(w, "failed to update settings", http.StatusInternalServerError)
			return
		}
	}

	if req.AIDescriptionPrompt != nil {
		if err := upsert("ai_description_prompt", *req.AIDescriptionPrompt); err != nil {
			httputil.JSONError(w, "failed to update settings", http.StatusInternalServerError)
			return
		}
	}

	if req.AIAPIKey != nil {
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `go test ./internal/settings/... -run TestUpdateAndGet -v`
Expected: PASS.

- [ ] **Step 5: Run the full settings package tests**

Run: `go test ./internal/settings/...`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/settings/handler.go internal/settings/handler_test.go
git commit -m "feat(settings): expose ai_description_prompt setting"
```

---

## Task 6: Populate `AIDescriptionPromptDefault` in GET response

**Files:**
- Modify: `internal/settings/handler.go`
- Modify: `internal/settings/handler_test.go`

The frontend needs the default string to show as placeholder. Source it from `pages.DefaultDescriptionPrompt`.

- [ ] **Step 1: Write a failing test**

Append to `internal/settings/handler_test.go`:

```go
func TestGetIncludesDescriptionPromptDefault(t *testing.T) {
	db := setupTestDB(t)
	h := NewHandler(db)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/settings", nil)
	h.Get(rr, req)

	var resp SettingsResponse
	json.NewDecoder(rr.Body).Decode(&resp)

	if resp.AIDescriptionPromptDefault == "" {
		t.Error("AIDescriptionPromptDefault should always be populated, got empty string")
	}
	if !strings.Contains(resp.AIDescriptionPromptDefault, "meta description") {
		t.Errorf("AIDescriptionPromptDefault = %q, expected to reference meta description", resp.AIDescriptionPromptDefault)
	}
}
```

- [ ] **Step 2: Run the test and verify it fails**

Run: `go test ./internal/settings/... -run TestGetIncludesDescriptionPromptDefault -v`
Expected: FAIL — `AIDescriptionPromptDefault` is never set, so it is the empty string.

- [ ] **Step 3: Wire the constant into the Get handler**

Edit `internal/settings/handler.go`. Add this import to the existing import block:

```go
import (
	"database/sql"
	"encoding/json"
	"net/http"

	"mees.space/internal/httputil"
	"mees.space/internal/pages"
)
```

Then find the end of the `Get` method, specifically:

```go
	if resp.AIModel == "" {
		resp.AIModel = "claude-sonnet-4-6"
	}

	// Mask the API key for security — only show last 8 chars
```

Replace with:

```go
	if resp.AIModel == "" {
		resp.AIModel = "claude-sonnet-4-6"
	}

	resp.AIDescriptionPromptDefault = pages.DefaultDescriptionPrompt

	// Mask the API key for security — only show last 8 chars
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `go test ./internal/settings/... -run TestGetIncludesDescriptionPromptDefault -v`
Expected: PASS.

- [ ] **Step 5: Run all tests and make sure the full build still works**

Run: `go build ./... && go test ./...`
Expected: PASS. If `go build` fails with an import cycle error (`internal/pages` → `internal/settings`), check whether any file in `internal/pages` imports `mees.space/internal/settings`. If so, stop and raise the issue — the spec's mitigation is to move `DefaultDescriptionPrompt` to a new leaf package, but based on current code (`description.go` reads the DB directly, does not import the settings package), this should not occur.

- [ ] **Step 6: Commit**

```bash
git add internal/settings/handler.go internal/settings/handler_test.go
git commit -m "feat(settings): return DefaultDescriptionPrompt in GET response"
```

---

## Task 7: Frontend settings UI — add description prompt textarea

**Files:**
- Modify: `frontend/src/app/admin/settings/page.tsx`

Add a second textarea below "AI System Prompt". Default string shown as placeholder. "Reset to default" link clears the textarea (which on save becomes an empty string → server falls back to default at generation time).

- [ ] **Step 1: Update the settings page**

Edit `frontend/src/app/admin/settings/page.tsx`. Find the state declarations at the top of the component:

```tsx
  const [systemPrompt, setSystemPrompt] = useState("");
  const [apiKey, setApiKey] = useState("");
  const [model, setModel] = useState("claude-sonnet-4-6");
  const [saving, setSaving] = useState(false);
  const [message, setMessage] = useState("");
```

Replace with:

```tsx
  const [systemPrompt, setSystemPrompt] = useState("");
  const [descriptionPrompt, setDescriptionPrompt] = useState("");
  const [descriptionPromptDefault, setDescriptionPromptDefault] = useState("");
  const [apiKey, setApiKey] = useState("");
  const [model, setModel] = useState("claude-sonnet-4-6");
  const [saving, setSaving] = useState(false);
  const [message, setMessage] = useState("");
```

Then find the `useEffect` block:

```tsx
  useEffect(() => {
    apiFetch("/api/settings")
      .then((r) => r.json())
      .then((data) => {
        setSystemPrompt(data.ai_system_prompt || "");
        setModel(data.ai_model || "claude-sonnet-4-6");
        setApiKey("");
      });
  }, []);
```

Replace with:

```tsx
  useEffect(() => {
    apiFetch("/api/settings")
      .then((r) => r.json())
      .then((data) => {
        setSystemPrompt(data.ai_system_prompt || "");
        setDescriptionPrompt(data.ai_description_prompt || "");
        setDescriptionPromptDefault(data.ai_description_prompt_default || "");
        setModel(data.ai_model || "claude-sonnet-4-6");
        setApiKey("");
      });
  }, []);
```

Then find the `save` function:

```tsx
  const save = async () => {
    setSaving(true);
    const body: Record<string, string> = { ai_system_prompt: systemPrompt, ai_model: model };
    if (apiKey) {
      body.ai_api_key = apiKey;
    }
```

Replace with:

```tsx
  const save = async () => {
    setSaving(true);
    const body: Record<string, string> = {
      ai_system_prompt: systemPrompt,
      ai_description_prompt: descriptionPrompt,
      ai_model: model,
    };
    if (apiKey) {
      body.ai_api_key = apiKey;
    }
```

Then find the JSX block with the "AI System Prompt" textarea (starts `<div style={{ marginBottom: "24px" }}>` near line 78 and contains the textarea with `value={systemPrompt}`). Immediately after its closing `</div>` (the one that contains the help text "This is prepended as a system message to every AI request from the editor."), insert this new section before the Model section:

```tsx
      <div style={{ marginBottom: "24px" }}>
        <div
          style={{
            display: "flex",
            justifyContent: "space-between",
            alignItems: "center",
            marginBottom: "8px",
          }}
        >
          <label
            style={{
              color: "rgba(255,255,255,0.6)",
              fontSize: "0.8rem",
              textTransform: "uppercase",
              letterSpacing: "0.05em",
            }}
          >
            SEO Description Prompt
          </label>
          {descriptionPrompt && (
            <button
              type="button"
              onClick={() => setDescriptionPrompt("")}
              style={{
                background: "none",
                border: "none",
                color: "rgba(255,255,255,0.4)",
                fontSize: "0.75rem",
                fontFamily: "inherit",
                cursor: "pointer",
                padding: 0,
                textDecoration: "underline",
              }}
            >
              reset to default
            </button>
          )}
        </div>
        <textarea
          value={descriptionPrompt}
          onChange={(e) => setDescriptionPrompt(e.target.value)}
          placeholder={descriptionPromptDefault}
          rows={4}
          style={{
            width: "100%",
            background: "var(--background)",
            border: "1px solid rgba(255,255,255,0.15)",
            borderRadius: "4px",
            padding: "10px",
            color: "var(--color)",
            fontFamily: "inherit",
            fontSize: "0.85rem",
            lineHeight: "1.6",
            resize: "vertical",
            outline: "none",
          }}
        />
        <p
          style={{
            color: "rgba(255,255,255,0.3)",
            fontSize: "0.75rem",
            marginTop: "6px",
          }}
        >
          Used when generating meta descriptions. Leave empty to use the default.
        </p>
      </div>
```

- [ ] **Step 2: Type-check and build the frontend**

Run: `cd frontend && npm run build`
Expected: Build succeeds with no TypeScript errors.

- [ ] **Step 3: Manual verification via dev server**

Run: `cd frontend && npm run dev` in one terminal; `air` (or `go run ./cmd/server`) in another.

Visit `http://localhost:3000/admin/settings` (after logging in). Verify:
  1. A new "SEO Description Prompt" textarea appears between the existing AI System Prompt and Model sections.
  2. When empty, the placeholder shows the canonical default starting with "Write a meta description for a webpage...".
  3. Entering text and clicking Save persists — reload the page and the text reappears.
  4. "reset to default" link appears only when the field is non-empty; clicking it clears the textarea; saving after that persists empty, and reloading shows the placeholder again.

Then edit a page and click "Save & regenerate description" to verify regeneration still works end-to-end (spot-check that descriptions are still being generated).

- [ ] **Step 4: Commit**

```bash
git add frontend/src/app/admin/settings/page.tsx
git commit -m "feat(frontend): add SEO description prompt editor in settings"
```

---

## Self-Review Checklist

After all tasks complete:

- [ ] Spec coverage: every bullet in the spec's Design section is implemented:
  - Settings key `ai_description_prompt` (Task 5)
  - Exported `DefaultDescriptionPrompt` (Task 1)
  - `loadDescriptionPrompt()` helper and fallback in `Generate()` (Task 3)
  - `AIDescriptionPrompt` request/response fields (Task 5)
  - `AIDescriptionPromptDefault` in GET response (Task 6)
  - Textarea + placeholder + reset-to-default in UI (Task 7)
- [ ] Tests cover: custom prompt is used (Task 3), default used when unset (Task 4), default used when whitespace-only (Task 4), round-trip through settings API (Task 5), default exposed in GET (Task 6).
- [ ] All Go tests pass: `go test ./...`
- [ ] Frontend builds clean: `cd frontend && npm run build`
- [ ] Full backend build clean: `go build ./...`
