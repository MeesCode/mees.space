# SEO Bundle — Design

**Date:** 2026-04-23
**Status:** Approved, ready for implementation plan
**Scope:** Backend-heavy feature addition; small frontend editor change. Adds full SEO support to the site: per-page OG/meta/canonical tags, sitemap, robots.txt, and AI-generated descriptions.

## Goal

Deliver SEO that actually works for crawlers and social scrapers. Specifically:

1. **`sitemap.xml`** — served at `/sitemap.xml`, built from the `pages` table.
2. **`robots.txt`** — served at `/robots.txt`, points at the sitemap, disallows admin/API paths.
3. **Per-page `<title>`, `<meta description>`, OpenGraph tags, Twitter card tags, and `<link rel="canonical">`** — injected into the initial HTML served by the Go backend, so crawlers and social scrapers (LinkedIn, Slack, Twitter, Discord) see per-page metadata without running JavaScript.
4. **AI-generated descriptions** — a new `description` column on the `pages` table, populated via Claude Haiku on manual save. Backfilled for existing pages on server startup.

## Context

- The frontend is a Next.js 15 static export. The build emits a single `index.html` shell with a generic title/description baked in; all content URLs currently serve that same HTML, populated client-side via `GET /api/pages/{path}`.
- Consequence today: social previews and non-JS crawlers see the same generic title for every URL.
- The Go backend already owns page metadata in SQLite and serves `dist/*` via `cmd/server/main.go`'s `staticHandler`. It's the right place to inject per-page `<head>` tags.
- An existing RSS feed at `/feed.xml` hardcodes `"https://mees.space"`. This spec replaces that hardcode with a new `MEES_BASE_URL` config.
- The editor has auto-save (debounced) and manual save (button click). The user's choice: AI generation runs **only on manual save**, not auto-save — to avoid burning API calls on every keystroke-debounce.

## Out of Scope

- Changing the frontend architecture (SSR, build-time prerender). Static export stays.
- Per-page OG image overrides. All pages use `/mees.png`.
- Structured data (JSON-LD). Not requested; can add later.
- A `description` field in the admin editor UI for manual editing. Descriptions are fully automated — user doesn't see or edit them.
- Automated frontend tests (no harness exists; explicit non-goal).
- Internationalization of robots.txt / sitemap.

## Architecture

**New or modified components:**

| Component | Responsibility |
|-----------|----------------|
| `internal/config` (modified) | Add `BaseURL` field from `MEES_BASE_URL` env var, default `https://mees.space` |
| `migrations/007_add_description.up.sql` / `.down.sql` (new) | `ALTER TABLE pages ADD COLUMN description TEXT NOT NULL DEFAULT ''` (up); drop column (down) |
| `internal/pages/description.go` (new) | Generate page descriptions via Claude Haiku; fallback to content-derived snippet; expose a sync generator and a background backfill runner |
| `internal/pages/sitemap.go` (new) | Build `sitemap.xml` from DB (mirrors RSS shape) |
| `internal/pages/handler.go` (modified) | New `GetSitemap`; modified save handlers to call description generator on manual save |
| `internal/pages/rss.go` (modified) | Use configured base URL instead of hardcoded string |
| `internal/pages/model.go` (modified) | `PageRequest` grows `Manual *bool` field |
| `internal/seo/inject.go` (new) | Read `dist/index.html` at startup; expose `Inject(PageMeta) []byte` for per-page meta injection |
| `cmd/server/main.go` (modified) | Register `/sitemap.xml` and `/robots.txt`; wire description backfill runner; inject seo package into `staticHandler` |
| `frontend/src/app/admin/editor/page.tsx` (modified) | Save button label updated; save body includes `manual: true` flag |

**Data flow — content-page GET (`GET /blog/my-post`):**

1. Go `staticHandler` receives the request.
2. For a content path (not `/admin/*`, not `/api/*`, not a static asset file): look up the page via `pagesSvc.GetPage`. If published, call `seo.Inject(PageMeta{...})` to produce customized HTML and serve it. If unpublished, still inject but with `NoIndex: true`. If not found, serve the raw (unmodified) shell and let the client-side 404 handle UX.
3. Admin/login pages and unknown paths fall through to `seo.Raw()`.

**Data flow — manual save (PUT `/api/pages/{path}` with `manual: true`):**

1. Frontend sends PUT with `{title, content, show_date, published, created_at, manual: true}`.
2. Handler writes file + DB as today.
3. Because `manual: true`: handler calls `descGen.Generate(ctx, title, content)` synchronously (up to 10 seconds).
4. Handler writes the returned description into the `description` column.
5. Returns 200 with updated page data.

Auto-save omits `manual` → handler skips description generation. Same endpoint, branching on one field.

**Data flow — server startup:**

1. Normal init (DB migrate, config load, etc.).
2. `seo.NewInjector(cfg.DistDir)` reads `dist/index.html`; verifies it contains `</head>`; `log.Fatal` on missing anchor.
3. Launch background goroutine: `descGen.BackfillEmpty(ctx, db, pagesSvc)`. Scans for pages with empty `description`, iterates with a 500ms delay, writes results. Respects `ctx.Done()` for graceful shutdown.

## Configuration Changes

Add to `internal/config/config.go`:

```go
BaseURL string // MEES_BASE_URL, default "https://mees.space"
```

Environment variable precedence: `MEES_BASE_URL` env, then default. Add to `.env.example` with a comment.

The Anthropic API key is read from the `settings` table under key `ai_api_key` — the same place `internal/ai` loads it. This is a per-request read so the admin can rotate the key via the settings UI without a server restart. When the key is empty/missing, generation transparently falls back to the content-derived snippet.

## Database Migration

**`migrations/007_add_description.up.sql`:**

```sql
ALTER TABLE pages ADD COLUMN description TEXT NOT NULL DEFAULT '';
```

**`migrations/007_add_description.down.sql`:**

```sql
ALTER TABLE pages DROP COLUMN description;
```

Existing rows get an empty string, which is exactly the signal the backfill uses to find them. No data loss, reversible.

## Description Generator (`internal/pages/description.go`)

### Public API

```go
type Generator struct {
    db      *sql.DB        // reads ai_api_key from settings per call
    client  ClaudeClient   // interface for test injection (HTTP transport)
    timeout time.Duration  // 10s default
}

type ClaudeClient interface {
    CreateMessage(ctx context.Context, apiKey string, req ClaudeRequest) (string, error)
}

func NewGenerator(db *sql.DB, timeout time.Duration) *Generator

// Generate returns a description for the page.
// Always returns a non-empty string (AI result or fallback snippet).
// Errors are logged but never returned.
func (g *Generator) Generate(ctx context.Context, title, content string) string

// BackfillEmpty runs until context is canceled or all pages with empty
// descriptions have been processed. Safe to run in a background goroutine.
func (g *Generator) BackfillEmpty(ctx context.Context, db *sql.DB, svc *Service)
```

### Claude call

- **Model:** `claude-haiku-4-5-20251001`.
- **max_tokens:** 120.
- **temperature:** 0.3.
- **System prompt:** `"Write a meta description for a webpage. Output a single sentence, 130-160 characters, no quotes, no trailing punctuation other than a period. Describe what the reader will learn or get from the page, not meta-commentary about the page itself."`
- **User prompt:** `"Title: " + title + "\n\nContent:\n" + truncate(content, 4000)`
- Post-processing of the response:
  1. Trim whitespace.
  2. Strip leading/trailing quote characters if present (`"`, `'`, `"`, `"`).
  3. If longer than 160 chars: cut at the last word boundary ≤160.
  4. If empty after trimming: treat as error → fallback.

### Fallback (`contentSnippet`)

Pure function, no I/O. Behavior:

1. Remove fenced code blocks (```…```).
2. Remove markdown syntax tokens: leading `#`, `>`, `-`, `*`, `1.` at line starts; inline `*`, `_`, `` ` ``, `~~`; link syntax `[text](url)` → `text`; images `![alt](url)` → `alt`.
3. Collapse whitespace runs to single spaces; trim.
4. Take first 160 chars; if cutting mid-word, back up to last space.

Deterministic and fast. Used directly by tests to avoid stubbing the API.

### Timeout and error handling

- `Generate` reads `ai_api_key` from the `settings` table at the start of each call.
- If the key is empty/missing: log once and return `contentSnippet(content)` immediately (no API call).
- Otherwise, wrap the API call in `context.WithTimeout`.
- Any error (network, timeout, empty response, rate limit): `log.Printf("description: AI generation failed: %v", err)` and return `contentSnippet(content)`.
- The generator **never** returns empty. Callers can assume the result is always usable.

### Backfill

- Query for pages: `SELECT path, title FROM pages WHERE description = '' AND published = 1 ORDER BY updated_at DESC`.
- For each row, load content via `svc.GetPage(path)` to get the markdown body (the DB doesn't store content).
- Call `Generate`, then `UPDATE pages SET description = ? WHERE path = ?`.
- `time.Sleep(500 * time.Millisecond)` between iterations.
- On `ctx.Done()`: exit cleanly.
- Runs exactly once per server boot.

## HTML Meta Injection (`internal/seo/inject.go`)

### Public API

```go
type Injector struct {
    template []byte  // read-only after NewInjector
}

type PageMeta struct {
    Title        string
    Description  string
    CanonicalURL string
    OGImage      string  // full URL
    NoIndex      bool
}

func NewInjector(distDir string) (*Injector, error)
func (i *Injector) Inject(m PageMeta) []byte
func (i *Injector) Raw() []byte
```

### Startup verification

`NewInjector` reads `{distDir}/index.html`. If:

- The file is missing: return error with message instructing user to build the frontend.
- The literal `</head>` is missing: return error. `main.go` calls `log.Fatal` so the problem is caught at startup rather than silently shipping broken SEO.

### Per-request injection

```go
func (i *Injector) Inject(m PageMeta) []byte {
    fragment := buildFragment(m)
    return bytes.Replace(i.template, []byte("</head>"), append(fragment, []byte("</head>")...), 1)
}
```

Only the first `</head>` is replaced (the intended one — inline scripts never emit literal `</head>`).

### Fragment contents

All string interpolations pass through `html.EscapeString`. Rendered HTML structure:

```html
<title>{{.Title}}</title>
<meta name="description" content="{{.Description}}">
<link rel="canonical" href="{{.CanonicalURL}}">
<meta property="og:title" content="{{.Title}}">
<meta property="og:description" content="{{.Description}}">
<meta property="og:url" content="{{.CanonicalURL}}">
<meta property="og:image" content="{{.OGImage}}">
<meta property="og:type" content="article">
<meta name="twitter:card" content="summary_large_image">
<meta name="twitter:title" content="{{.Title}}">
<meta name="twitter:description" content="{{.Description}}">
<meta name="twitter:image" content="{{.OGImage}}">
{{if .NoIndex}}<meta name="robots" content="noindex">{{end}}
```

Browsers resolve conflicting `<title>` and `<meta name="description">` elements by using the **last** one; since our injected pair appears after the Next.js-baked pair, our values win.

## `staticHandler` Integration

The existing function signature changes from:

```go
func staticHandler(distDir string) http.HandlerFunc
```

to:

```go
func staticHandler(distDir, baseURL string, pagesSvc *pages.Service, injector *seo.Injector) http.HandlerFunc
```

Behavior — added after the existing static-file fallbacks:

1. **Known static-file paths** (images, `_next/*`, etc.): serve as today. No meta injection.
2. **Admin paths** (`/admin/*`): if the file exists in `dist/admin/`, serve raw — `injector.Raw()` is NOT applied because Next.js' own admin HTML already has its own head structure that we don't want to touch.
3. **Content path fallback** (what currently falls through to `index.html`): compute the page path (empty → `home`), look up in pages service:
   - Published page → inject full meta, serve customized HTML.
   - Unpublished page → inject meta with `NoIndex: true`. Title/description use a generic placeholder (`"Draft — Mees Brinkhuis"`, no leakage).
   - Not found → serve `injector.Raw()` unchanged so the client-side 404 UX runs.
4. **Home page** (`/`): treated as a content path with `path = "home"`. Canonical URL is `baseURL` (no trailing slash, no path suffix).

## Sitemap (`internal/pages/sitemap.go`)

### Public API

```go
func BuildSitemap(db *sql.DB, baseURL string) ([]byte, error)
```

### Query

```sql
SELECT path, updated_at FROM pages WHERE published = 1 ORDER BY updated_at DESC
```

### Output structure

```xml
<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url>
    <loc>{baseURL}</loc>
    <lastmod>{YYYY-MM-DD}</lastmod>
  </url>
  <url>
    <loc>{baseURL}/{path}</loc>
    <lastmod>{YYYY-MM-DD}</lastmod>
  </url>
  ...
</urlset>
```

- Path `home` renders as bare `{baseURL}` (no `/home`).
- `<lastmod>` is `updated_at` formatted `YYYY-MM-DD` (sitemap spec allows date-only).
- No `<priority>` or `<changefreq>` — both are legacy, largely ignored.

### Handler

`GetSitemap` in `internal/pages/handler.go`, mirrors `GetRSS`:
- `Content-Type: application/xml; charset=utf-8`
- `Cache-Control: public, max-age=3600`

Registered in `main.go`: `mux.HandleFunc("GET /sitemap.xml", pagesHandler.GetSitemap)`.

## robots.txt

Inline handler in `main.go`:

```go
mux.HandleFunc("GET /robots.txt", func(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "text/plain; charset=utf-8")
    w.Header().Set("Cache-Control", "public, max-age=3600")
    fmt.Fprintf(w, "User-agent: *\nDisallow: /admin/\nDisallow: /api/\nAllow: /\n\nSitemap: %s/sitemap.xml\n", cfg.BaseURL)
})
```

## Frontend Editor Changes

### Manual save button label

In `frontend/src/app/admin/editor/page.tsx`, the Save button's label changes from:

```tsx
{saving ? "Saving..." : "Save"}
```

to:

```tsx
{saving ? "Saving & describing…" : "Save & regenerate description"}
```

The `saving` state already drives disable + spinner text — no new state.

### `savePage` includes `manual: true`

The fetch body inside `savePage` gains one field:

```tsx
body: JSON.stringify({
  title,
  content,
  show_date: showDate,
  published,
  created_at: createdAt,
  manual: true,   // new — triggers description regeneration
}),
```

The auto-save path (around line 68) stays unchanged and does not send `manual`.

### Type update

The frontend doesn't have a dedicated `PageRequest` type in `lib/types.ts` — the save body is an inline object literal in `savePage`. Only change needed client-side: add `manual: true` to that literal (shown above).

Backend `PageRequest` in `internal/pages/model.go` gains `Manual *bool` with JSON tag `manual,omitempty`. Pointer type so absence is distinguishable from `false` (auto-save should be treated exactly the same as a legacy client that doesn't send the field).

## Testing

**Go tests** (required):

- `internal/pages/sitemap_test.go`: seed published + unpublished pages, assert XML shape and inclusion/exclusion. Table-driven across a few shapes (home only, home + regular, drafts excluded).
- `internal/pages/description_test.go`: test `contentSnippet` against a set of markdown inputs (prose only, with code fences, with list markers, with links); test `Generate` with a stub `ClaudeClient` returning canned response; test `Generate` with a stub returning error → asserts fallback. No network.
- `internal/seo/inject_test.go`: load a minimal HTML fixture, inject `PageMeta`, assert each expected tag present with correct attributes. One test for `NoIndex: true`; one with values containing `<script>` and `"` to confirm escaping; one fixture without `</head>` asserting `NewInjector` returns an error.
- `internal/config/config_test.go`: extend to cover `BaseURL` default and env-var override.
- `internal/pages/handler_test.go`: add a small test for `GetSitemap` (correct Content-Type and XML body shape). If `GetRobots` is inlined in `main.go`, it is covered by manual smoke test instead.

**Manual browser verification** (per-task, after backend+frontend changes):

1. Restart the server. Confirm startup log shows the seo injector initialized and backfill starting.
2. Open an existing page in the admin editor; click Save. Confirm button reads "Saving & describing…" and spinner persists until the call returns. Confirm save succeeds.
3. `curl -s http://localhost:8080/some-page | grep -E 'og:|canonical|description' | head -10` — expect per-page values.
4. `curl -s http://localhost:8080/sitemap.xml | xmllint --format -` — expect URL list.
5. `curl -s http://localhost:8080/robots.txt` — expect the 4-line robots output with the Sitemap line.
6. `curl -s -A "facebookexternalhit/1.1" http://localhost:8080/some-page | grep og:` — confirms scrapers see the tags.
7. Visit an unpublished draft URL (requires auth or known path) and confirm the injected HTML contains `<meta name="robots" content="noindex">`.
8. Verify admin pages (`/admin/editor`) are served without injected meta (check view-source).

**Not covered:**
- Actual social previews in LinkedIn/Slack/Discord (requires public-facing deployment; `curl -A` is the in-repo approximation).
- End-to-end test of the Claude API call (network-dependent, not worth the flakiness).

## Risks & Non-Concerns

- **Anchor brittleness:** the `</head>` literal replacement depends on the shell not embedding `</head>` inside an inline script. Next.js's output currently doesn't, and the startup verification would fail loudly if this ever changed.
- **API key absence:** when the `ai_api_key` setting is empty/unset, generation transparently falls back to `contentSnippet`. The site remains functional; only the quality of descriptions degrades. Backfill still runs and produces reasonable content-derived descriptions.
- **Description backfill concurrency:** the backfill goroutine and on-save updates could race on the same row. Accepted — SQLite's UPDATE is serialized, and the last writer wins. The worst outcome is a slightly stale description for a few seconds.
- **Cost:** backfill runs at most once per boot. On-save only for manual saves. Haiku 4.5 at ~$0.0001 per call. Trivial for a personal blog.
- **RSS hardcode:** replaced with `cfg.BaseURL` in this spec. If anything else in the codebase hardcodes the URL, we'll find and fix during implementation.
