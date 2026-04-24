# SSR Content + Heading Anchors — Design

**Date:** 2026-04-24
**Status:** Approved, ready for implementation plan
**Scope:** Go backend (`internal/render`, `internal/seo`, `cmd/server`) + frontend (`ContentPage`, `MarkdownRenderer`, `globals.css`).

## Goal

Make fragment-URL navigation (`/post#section`) scroll correctly on **first page load**, and reintroduce hover-reveal clickable heading anchors. The previous attempt at heading anchors was reverted (commit `120aaaf`) because `ContentPage` fetches markdown client-side — by the time the DOM contains the heading, the browser's native scroll-to-fragment has already fired and quietly failed. This spec fixes the root cause by server-rendering the page content into the HTML shell on the first request, so the heading element exists before the browser looks for the fragment target.

The previous client-side piece (`rehype-slug` + `rehype-autolink-headings`, hover-reveal CSS) is reintroduced unchanged and covers the SPA-navigation path. The new server-rendered path covers first paint.

## Context

- Today, `cmd/server/main.go`'s `staticHandler` serves `dist/index.html` via `seo.Injector`, which inserts per-page `<head>` metadata but leaves the body as the static Next.js-exported shell (loading state with "Loading…").
- `frontend/src/app/[[...slug]]/ContentPage.tsx` fetches `/api/pages/{path}`, then renders via `MarkdownRenderer` (`react-markdown` + `remark-gfm` + `rehype-raw` + `rehype-highlight`).
- The PR that added client-side heading anchors (spec `2026-04-23-reading-time-and-heading-anchors-design.md`, plan `2026-04-23-reading-time-and-heading-anchors.md`) was reverted because first-load fragment scroll was broken. The reading-time / page-meta portion of that work is still shipped and is untouched by this spec.
- `output: "export"` in `next.config.ts` means only one HTML file (`dist/index.html`) is produced. All routing is client-side. Server-rendered content must be inserted into that single shell at request time.

## Out of Scope

- Server-side syntax highlighting (Chroma / goldmark-highlighting). Code blocks render unstyled on first paint; `highlight.js` decorates them after mount. Server highlighting is a follow-up.
- Server-rendering for `/admin/*` routes. Admin paths continue to use the raw shell (unchanged in `cmd/server/main.go`).
- Server-rendering content for unpublished (draft) pages. Public requests for drafts keep their current no-content shell with `noindex` meta, to avoid leaking draft content.
- Table of contents.
- A second HTTP route or query param to opt in/out of SSR.
- Any Next.js `app router` changes beyond the `ContentPage` tweak. We stay on static export.

## Architecture

Two rendering paths share one source of truth (`internal/pages.Service`):

| Path | Renderer | Trigger |
|------|----------|---------|
| First paint | `internal/render.MarkdownToHTML` (goldmark) in Go | Every non-admin GET request to the static handler |
| SPA navigation | `react-markdown` via `MarkdownRenderer` in React | User clicks a link handled by `navigation.navigate()` |

The two renderers must produce matching heading IDs so fragment links keep working whether the heading was rendered server-side (first load) or client-side (SPA nav). We enforce parity via a shared slug algorithm and a shared test-vector list (see "Heading ID parity" below).

### Request flow — first paint

```
GET /my-post
  staticHandler
    pagesSvc.GetPage("my-post")            // existing
    render.MarkdownToHTML(page.Content)     // NEW: bytes of rendered HTML
    json.Marshal(page)                      // existing PageResponse shape
    seo.Injector.Inject(meta, bodyHTML, pageDataJSON)  // EXTENDED
  → dist/index.html with:
      • existing <head> SEO tags
      • <div id="content">{rendered HTML}</div> replacing the loading placeholder
      • <script id="__page_data__" type="application/json">{page JSON}</script>
```

### Browser — first paint

1. HTML parsed. Headings with `id` attrs exist. Native scroll-to-fragment fires and lands on `#section`.
2. React boots. `ContentPage` reads `__page_data__` synchronously during initial state init.
3. If bootstrap matches current path (`window.location.pathname` → `pagePath`), initial state is `{ page, loading: false, ssr: true }`. Initial render emits `<div id="content" dangerouslySetInnerHTML={{ __html: ssrHTML }} />`. The server-rendered DOM is preserved byte-for-byte → no hydration mismatch, no flash, no reflow.
4. `useEffect` runs once after mount: `hljs.highlightAll()` on the content div (decorates code blocks), and if `window.location.hash` is set, `scrollIntoView` on the target (defensive — covers cases where late-loading images shifted layout).
5. `POST /api/views/{path}` still fires.

### Browser — SPA navigation

1. User clicks an internal link → `navigation.navigate(href)` → `pushState` + `setPath`.
2. `ContentPage`'s fetch effect runs because `pagePath` changed → `fetch /api/pages/{newPath}` → state update → re-render via `MarkdownRenderer` (react-markdown path).
3. After content renders: `if (hash) scrollIntoView(target) else scrollTo(0, 0)`. This replaces the current unconditional `scrollTo(0, 0)` so SPA-level fragment navigation also works.

## Components

### 1. `internal/render` (new package)

**File:** `internal/render/markdown.go`

**Public API:**

```go
// Renderer is concurrency-safe and should be constructed once at startup.
type Renderer struct { /* goldmark.Markdown */ }

func New() *Renderer

// ToHTML renders CommonMark + GFM to HTML with heading ids and anchor links.
func (r *Renderer) ToHTML(src []byte) ([]byte, error)
```

**Pipeline:**

- `goldmark.New()` with:
  - `extension.GFM` (tables, strikethrough, autolinks, task lists — matches `remark-gfm`)
  - `html.WithUnsafe()` (matches `rehype-raw` — embedded HTML passes through)
  - custom `AST` transformer that assigns an `id` to every `ast.Heading` (levels 1–6) using our shared slug function, then appends an `<a class="heading-anchor" href="#{id}" aria-hidden="true" tabindex="-1">#</a>` node as the **first child** of the heading (matches `rehype-autolink-headings` with `behavior: "prepend"`).

**Slug function:** `internal/render/slug.go`, exported `Slugify(text string, seen map[string]int) string`. Algorithm mirrors `github-slugger` (what `rehype-slug` uses):

1. Lowercase.
2. Strip anything that is not a Unicode letter, number, hyphen, underscore, or space.
3. Replace runs of whitespace with single hyphens.
4. If the resulting slug is already in `seen`, append `-1`, `-2`, … until unique; update `seen`.

A compact `slug_test.go` asserts parity against a **shared test-vector JSON file** (`internal/render/testdata/slug_vectors.json`) that the frontend build also exercises (see Task 3 below).

**Why goldmark:** pure Go, CommonMark + GFM parity, AST hooks make the anchor transform trivial. No CGO, no Ruby/JS dependency.

### 2. `internal/seo` (extend)

**File:** `internal/seo/inject.go`

Extend `Injector` with:

```go
// Inject was:  func (i *Injector) Inject(m PageMeta) []byte
// Now:         func (i *Injector) Inject(m PageMeta, body BodyInjection) []byte

type BodyInjection struct {
    HTML []byte // rendered markdown; empty → leave loading placeholder intact
    Data []byte // JSON-marshaled PageResponse; empty → no bootstrap script
}
```

- `NewInjector` additionally verifies two new anchors in `dist/index.html`:
  - Content slot: `<!--SSR_CONTENT-->` (placed by React inside the loading `<div id="content">`, see Task 4). Presence required.
  - Data slot: `<!--SSR_DATA-->` (placed by `layout.tsx` just before closing `<body>` via a tiny inert component, see Task 4). Presence required.
- On `Inject`, the two comment markers are string-replaced with the provided HTML / script. If a slot's payload is empty, the marker is replaced with the empty string (not left in place, to avoid leaking markers to the browser).
- `Raw()` (used for admin shell) replaces both markers with empty strings so admin HTML has no SSR artefacts.

### 3. Shared slug test vectors

**File:** `internal/render/testdata/slug_vectors.json`

A list of `{ "input": string, "history": string[], "want": string }` triples. Go tests consume this directly. The frontend adds a **build-time parity test** (`frontend/scripts/verify-slug-parity.ts`) that imports `github-slugger` (same engine `rehype-slug` uses) and asserts each vector round-trips to the same output. The script is invoked from `npm run build` via a pre-build step (`"prebuild": "tsx scripts/verify-slug-parity.ts"`). Either renderer diverging from the other breaks `make build`.

Initial vector list covers: basic ASCII, mixed case, punctuation, leading/trailing spaces, non-ASCII letters (é, ü, 中), duplicate handling (3 runs of the same input → `foo`, `foo-1`, `foo-2`), empty string edge case.

### 4. Frontend — `MarkdownRenderer` + `ContentPage` + `layout.tsx`

**`frontend/src/components/MarkdownRenderer.tsx`** — reinstate the pre-revert pipeline: add `rehype-slug` and `rehype-autolink-headings` with the prior configuration (`behavior: "prepend"`, className `heading-anchor`, `aria-hidden`, `tabIndex: -1`, text content `"#"`). Identical to what commit `120aaaf` removed.

**`frontend/src/app/globals.css`** — reinstate the `.heading-anchor` + per-heading hover rules that commit `120aaaf` removed. Identical to the pre-revert styles.

**`frontend/src/app/[[...slug]]/ContentPage.tsx`** — three changes:

1. **SSR content slot in the loading state.** Change the loading return from `<article id="content"><p>Loading…</p></article>` to `<div id="content" dangerouslySetInnerHTML={{ __html: "<!--SSR_CONTENT-->" }} />`. This has three effects: (a) the static export emits a stable marker that the Go injector can find; (b) React will not touch this node during hydration when the marker is present (it honours `dangerouslySetInnerHTML`); (c) during `npm run dev` (where Go isn't in front of Next), the loading state is visually empty — the previous "Loading…" text moves into the `!ssr` branch (see below) where we do a plain fetch.

2. **Bootstrap-aware initial state.** Add a module-level helper `readBootstrap()` that once per module-load reads `document.getElementById("__page_data__")`, parses its JSON, and caches the result in a module variable. `ContentPage` initialises state as:

   ```ts
   const bootstrap = readBootstrap();
   const usedBootstrap = bootstrap !== null && bootstrapPath(bootstrap) === pagePath;
   const [page, setPage] = useState<PageData | null>(usedBootstrap ? bootstrap : null);
   const [loading, setLoading] = useState(!usedBootstrap);
   const [ssr, setSsr] = useState(usedBootstrap);
   ```

   Where `bootstrapPath(p)` returns `p.path`. The `usedBootstrap` branch is consumed on the first mount and cleared (`readBootstrap.consumed = true`) so subsequent in-app navigations always take the fetch path. The fetch `useEffect` checks `ssr` and skips the network call on the first run; subsequent path changes set `ssr = false` and fetch normally.

3. **Hash-aware scroll.** Replace the current unconditional `window.scrollTo(0, 0)` with:

   ```ts
   const hash = window.location.hash.slice(1);
   if (hash) {
     document.getElementById(hash)?.scrollIntoView();
   } else {
     window.scrollTo(0, 0);
   }
   ```

   Runs in both the SSR-initial branch (after mount) and the post-fetch branch.

4. **Rendering branch.** When `ssr && page`: render `<div id="content" dangerouslySetInnerHTML={{ __html: markdownRenderedByServerAndEmbeddedInBootstrap }} />`. The raw rendered HTML string is the field `page.rendered_html` on the bootstrap (see "Bootstrap JSON shape" below). When `!ssr && page`: render the existing `<MarkdownRenderer content={page.content} />`. Same wrapping `<div className={page.show_date ? "has-meta" : ""}>` either way.

**`frontend/src/app/layout.tsx`** — insert the data-slot marker at the end of `<body>`:

```tsx
<body>
  <Providers>
    <ClientLayout>{children}</ClientLayout>
  </Providers>
  <div
    id="__ssr_data_slot__"
    dangerouslySetInnerHTML={{ __html: "<!--SSR_DATA-->" }}
    aria-hidden="true"
    style={{ display: "none" }}
  />
</body>
```

React leaves this alone forever; the Go server swaps the comment for a real `<script id="__page_data__" type="application/json">…</script>`. In dev (Next serves directly), the marker stays inert.

### 5. Bootstrap JSON shape

The `PageResponse` Go struct (`internal/pages/model.go`) gains one new field, populated only for the bootstrap payload (not for `/api/pages` responses, which stay byte-compatible):

```go
type PageResponse struct {
    // existing fields…
    RenderedHTML string `json:"rendered_html,omitempty"`
}
```

We populate `RenderedHTML` only in `cmd/server/main.go` `serveContentPage` right before marshalling for the bootstrap. The `/api/pages/{path}` handler continues to omit it (default `""`), so API consumers and admin are unaffected.

### 6. `cmd/server/main.go`

`serveContentPage` signature extended to receive the `render.Renderer`. Flow becomes:

1. `page, err := pagesSvc.GetPage(pagePath)` — unchanged.
2. For published pages only: `html, err := renderer.ToHTML([]byte(page.Content))`. On error, log and fall through with empty `html` so the SPA fetch path still works; never 500.
3. Build `PageResponse`, set `RenderedHTML = string(html)`, marshal to JSON.
4. `body := seo.BodyInjection{ HTML: html, Data: jsonBytes }`.
5. `w.Write(injector.Inject(meta, body))`.

For drafts / non-publicly-visible pages we still inject `meta` (`NoIndex`, generic title) and pass `body` with empty `HTML`/`Data`. The client will fall back to its authed fetch path, same as today.

For the 404 path (`pagesSvc.GetPage` returns not-found) we pass `body` with empty slots. The client's 404 UX in `ContentPage` still runs.

`staticHandler` for admin paths keeps calling `injector.Raw()`, which now strips the markers to empty strings — admin HTML is functionally unchanged.

## Data flow diagram

```
┌──────── Go (request time) ────────┐
│ pages.Service ──► content bytes   │
│      │                            │
│      ▼                            │
│ render.Renderer ──► HTML + ids    │
│      │                            │
│      ▼                            │
│ PageResponse JSON (with HTML)     │
│      │                            │
│      ▼                            │
│ seo.Injector ──► dist/index.html  │
│   inserts: SEO meta, content,     │
│            bootstrap script       │
└───────────────────────────────────┘
              │
              ▼  first paint
┌──────── Browser ──────────────────┐
│ HTML parse ──► native #hash scroll│
│      │                            │
│      ▼                            │
│ React hydrate (bootstrap state)   │
│   ContentPage.render = SSR div    │
│   useEffect: hljs + scrollIntoView│
│                                   │
│ later: SPA nav                    │
│   fetch /api/pages/{p} ──►        │
│   react-markdown render ──►       │
│   hash-aware scroll               │
└───────────────────────────────────┘
```

## Heading ID parity — how we enforce it

Parity between Go's goldmark transform and JS's `rehype-slug` is a correctness-critical invariant (deep-link URLs must survive a transition between the two renderers). We achieve it by:

1. Shipping our own `Slugify` in Go rather than relying on goldmark's built-in auto-id extension (which has subtly different rules).
2. Basing its algorithm on `github-slugger` (exact engine used by `rehype-slug`).
3. Sharing a JSON test-vector file between Go (`go test ./internal/render`) and frontend (`npm run build` pre-step), failing the build on divergence.
4. Keeping the duplicate-suffix counter behaviour identical: first occurrence = bare slug, second = `-1`, third = `-2`, etc.

A divergence is a compile-time failure, not a runtime surprise.

## Error handling

- **goldmark render failure.** Log with page path, proceed with empty `html`. The client fetch path already handles empty content (shows Loading then content once fetched). We never 500 the shell because of a content bug.
- **`json.Marshal` failure** on the bootstrap. Log and proceed with empty `Data`; client falls back to fetch. Realistically unreachable for `PageResponse`.
- **Missing anchors in `dist/index.html`.** `NewInjector` already fails fast on a missing `</head>`; we extend that check to the two new markers. Build-time regression → startup crash, which is what we want.
- **Bootstrap JSON parse failure on client.** `readBootstrap()` returns `null`; `ContentPage` takes the fetch path. Logged via `console.warn` with the parse error for dev visibility.
- **Bootstrap path mismatch** (e.g., user hit a route that somehow served a stale shell). `ContentPage` falls through to the fetch path. No user-visible error.
- **Draft leak guard.** `serveContentPage` never populates `body.HTML` or `body.Data` for unpublished pages. Unit test asserts this.

## Verification plan

### Automated

- **Go unit tests (`internal/render/markdown_test.go`):** assert goldmark output contains `<h2 id="foo"><a class="heading-anchor" …>#</a>…</h2>` for a representative markdown corpus. GFM table, code fence with language class, inline raw HTML, image, link, nested headings with duplicate text.
- **Go unit tests (`internal/render/slug_test.go`):** every vector in `testdata/slug_vectors.json` round-trips to the expected slug, including duplicate-counter sequences.
- **Go unit tests (`internal/seo/inject_test.go`):** `Inject` correctly replaces both markers, `Raw` strips both, `NewInjector` fails when either marker is missing.
- **Go integration test (`cmd/server/main_test.go` or new `*_ssr_test.go`):** boot a server against a temp content dir, GET `/some-post` → response body contains the page's rendered markdown inside `<div id="content">` and a `<script id="__page_data__">` with the page JSON; draft page → neither; 404 path → neither, status 404.
- **Frontend parity test (`frontend/scripts/verify-slug-parity.ts`):** runs on every `npm run build`, asserts `github-slugger` matches the Go test vectors.

### Manual

- `make build-run`, load `/some-post#section` cold (clear cache, new tab). Browser scrolls to `#section` on first paint with no visible jump. View-source shows the content and the bootstrap script.
- Same URL in a no-JS browser (Safari Reader, `curl`) shows real content.
- Click an in-app link to `/other-post#other-section`. SPA nav: scroll lands on `#other-section`. Hover any heading, `#` fades in; click copies the URL with fragment.
- Load a draft page as an authed admin. SSR slot is empty, client fetch populates content, heading anchors present (via client path).
- Load a non-existent path. 404 UX renders from `ContentPage`. HTTP status is 404 (already implemented).
- Admin routes (`/admin/editor`, `/admin/settings`, `/admin/login`) load and function identically to today. View-source shows no residual `<!--SSR_CONTENT-->` / `<!--SSR_DATA-->` markers.

## Dependencies

- Go: `github.com/yuin/goldmark` (MIT, pure Go, widely used).
- Frontend: `rehype-slug`, `rehype-autolink-headings` (same versions the reverted work used), plus a dev-only `tsx` if not already present for the parity script.

## Risks & Non-Concerns

- **Server CPU per request.** Rendering a typical post (~5 KB markdown) with goldmark is sub-millisecond. Negligible vs. database read + disk I/O already in the path. Not worth caching yet; revisit if a flame graph disagrees.
- **HTML size.** Each request now carries rendered content twice (once in the DOM, once in the JSON bootstrap). For a 5 KB post that's ~10 KB extra on the wire; for a 50 KB post, ~100 KB. Acceptable for a personal site; if a post ever gets big enough to matter, deduplicate by having the bootstrap reference the DOM instead of carrying a second copy.
- **React hydration semantics across Next versions.** The `dangerouslySetInnerHTML` + matching-payload pattern is stable across React 18 and 19 and is how most SSR + client-render coexistence has worked for years. Low risk.
- **Parity drift.** Bounded by the shared-vector build check. A new slug vector or a goldmark version bump either passes the build or fails it loudly.
- **Admin editor preview.** `MarkdownRenderer` renders with anchors in admin preview (unchanged). The SSR layer does not touch admin, so no risk of leaking rendered HTML into admin flows.
