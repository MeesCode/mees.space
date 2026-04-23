# Polish Bundle Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Return proper HTTP 404 on missing content paths (preserving the existing inline 404 UX), and add native `loading="lazy"` to markdown-rendered images + sidebar social icons.

**Architecture:** Two independent changes. Backend edit in `cmd/server/main.go`'s `serveContentPage` plus a new `cmd/server/main_test.go` exercising the 200 and 404 branches via `httptest`. Frontend edits to `MarkdownRenderer.tsx` (components-prop `img` override) and `Sidebar.tsx` (social-icon attributes).

**Tech Stack:** Go 1.26+ (`net/http`, `httptest`), Next.js 15 static export, React 19, TypeScript.

**Reference spec:** `docs/superpowers/specs/2026-04-23-polish-bundle-design.md`

**Notes for implementers:**
- Node 20 via nvm, wrap `npm` in `bash -c 'export NVM_DIR="$HOME/.nvm"; source "$NVM_DIR/nvm.sh"; nvm use 20 > /dev/null; cd <path>; <command>'`.
- Backend tests: `go test ./...` from worktree root.
- `seo.Injector` needs `dist/index.html` to contain `</head>`; tests write a minimal fixture to a tmpdir.

---

## File Map

**Create:**
- `cmd/server/main_test.go` — two-case table test for `serveContentPage` via `httptest`.

**Modify:**
- `cmd/server/main.go` — update `serveContentPage` error branch to set HTTP 404 + inject a generic no-index shell.
- `frontend/src/components/MarkdownRenderer.tsx` — add `components={{ img: … }}` override.
- `frontend/src/components/Sidebar.tsx` — add `loading="lazy"` to the two social icons.

---

## Task 1: Proper HTTP 404 on missing content paths (backend)

**Files:**
- Modify: `cmd/server/main.go` (function `serveContentPage`)
- Create: `cmd/server/main_test.go`

- [ ] **Step 1: Write the failing test**

Create `cmd/server/main_test.go`:

```go
package main

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"

	"mees.space/internal/pages"
	"mees.space/internal/seo"
)

func setupContentPageTest(t *testing.T) (*pages.Service, *seo.Injector) {
	t.Helper()
	tmp := t.TempDir()

	// DB with the pages schema the service expects.
	dbPath := filepath.Join(tmp, "test.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	_, err = db.Exec(`CREATE TABLE pages (
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
	if err != nil {
		t.Fatal(err)
	}

	// Content dir with a "home.md" file so GetPage("home") succeeds.
	contentDir := filepath.Join(tmp, "content")
	if err := os.MkdirAll(contentDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(contentDir, "home.md"), []byte("# Home\n\nBody."), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO pages (path, title) VALUES ('home', 'Home Page')`); err != nil {
		t.Fatal(err)
	}

	// Minimal dist/index.html with the </head> anchor.
	distDir := filepath.Join(tmp, "dist")
	if err := os.MkdirAll(distDir, 0755); err != nil {
		t.Fatal(err)
	}
	shell := "<!doctype html><html><head><title>Default</title></head><body></body></html>"
	if err := os.WriteFile(filepath.Join(distDir, "index.html"), []byte(shell), 0644); err != nil {
		t.Fatal(err)
	}

	injector, err := seo.NewInjector(distDir)
	if err != nil {
		t.Fatal(err)
	}
	svc := pages.NewService(db, contentDir)
	return svc, injector
}

func TestServeContentPageFound(t *testing.T) {
	svc, injector := setupContentPageTest(t)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/home", nil)
	serveContentPage(w, r, "home", "https://example.com", svc, injector)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "<title>Home Page</title>") {
		t.Errorf("body missing page title; got:\n%s", body)
	}
}

func TestServeContentPageNotFound(t *testing.T) {
	svc, injector := setupContentPageTest(t)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/does-not-exist", nil)
	serveContentPage(w, r, "does-not-exist", "https://example.com", svc, injector)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, `<meta name="robots" content="noindex">`) {
		t.Errorf("body missing noindex meta; got:\n%s", body)
	}
	if !strings.Contains(body, "Not Found — Mees Brinkhuis") {
		t.Errorf("body missing generic Not Found title; got:\n%s", body)
	}
}
```

- [ ] **Step 2: Run tests to verify failure**

```bash
cd /home/mees/git/mees.space/.worktrees/polish-bundle
go test ./cmd/server/ -run TestServeContentPage -v
```
Expected: `TestServeContentPageNotFound` FAILS because current `serveContentPage` writes the raw shell with implicit 200.

- [ ] **Step 3: Update `serveContentPage` in `cmd/server/main.go`**

Find the current `serveContentPage` function. The current error branch looks like:

```go
page, err := pagesSvc.GetPage(pagePath)
if err != nil {
    // Not found or invalid — fall back to raw shell for client-side 404
    writeHTML(w, injector.Raw())
    return
}
```

Replace the error branch with:

```go
page, err := pagesSvc.GetPage(pagePath)
if err != nil {
    // Missing / invalid path — inject a generic no-index shell with HTTP 404
    // so crawlers and monitoring treat this as a proper 404. The client-side
    // 404 UX in ContentPage.tsx still renders for human visitors.
    meta := seo.PageMeta{
        Title:       "Not Found — Mees Brinkhuis",
        Description: "",
        NoIndex:     true,
    }
    w.Header().Set("Content-Type", "text/html; charset=utf-8")
    w.WriteHeader(http.StatusNotFound)
    w.Write(injector.Inject(meta))
    return
}
```

Do NOT change anything else in the function or file.

- [ ] **Step 4: Run tests to verify pass**

```bash
go test ./cmd/server/ -run TestServeContentPage -v
go test ./...
```
Expected: both new tests PASS; full suite PASSES.

- [ ] **Step 5: Commit**

```bash
git add cmd/server/main.go cmd/server/main_test.go
git commit -m "fix(server): return HTTP 404 for missing content paths"
```

---

## Task 2: Lazy loading for markdown-rendered images

**Files:**
- Modify: `frontend/src/components/MarkdownRenderer.tsx`

- [ ] **Step 1: Replace the contents of `MarkdownRenderer.tsx`**

The current file (after the anchor-link revert) looks like:

```tsx
"use client";

import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import rehypeRaw from "rehype-raw";
import rehypeHighlight from "rehype-highlight";
import "highlight.js/styles/atom-one-dark.css";

interface Props {
  content: string;
}

export function MarkdownRenderer({ content }: Props) {
  return (
    <div id="content">
      <ReactMarkdown remarkPlugins={[remarkGfm]} rehypePlugins={[rehypeRaw, rehypeHighlight]}>
        {content}
      </ReactMarkdown>
    </div>
  );
}
```

Replace it with (adds `components` prop overriding `img`):

```tsx
"use client";

import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import rehypeRaw from "rehype-raw";
import rehypeHighlight from "rehype-highlight";
import "highlight.js/styles/atom-one-dark.css";

interface Props {
  content: string;
}

export function MarkdownRenderer({ content }: Props) {
  return (
    <div id="content">
      <ReactMarkdown
        remarkPlugins={[remarkGfm]}
        rehypePlugins={[rehypeRaw, rehypeHighlight]}
        components={{
          img: (props) => <img {...props} loading="lazy" decoding="async" />,
        }}
      >
        {content}
      </ReactMarkdown>
    </div>
  );
}
```

- [ ] **Step 2: Verify build**

```bash
bash -c '
export NVM_DIR="$HOME/.nvm"
source "$NVM_DIR/nvm.sh"
nvm use 20 > /dev/null
cd /home/mees/git/mees.space/.worktrees/polish-bundle/frontend
npm run build 2>&1 | tail -10
'
```
Expected: build succeeds, no TS errors.

If TypeScript complains about the `img` override's props type, replace the `components` block with an explicit type cast — but `react-markdown@10` infers `img` props correctly from its `Components` type and the simple `(props) => <img {...props} ... />` form should compile as-is.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/components/MarkdownRenderer.tsx
git commit -m "feat(frontend): lazy-load markdown-rendered images"
```

---

## Task 3: Lazy loading for sidebar social icons

**Files:**
- Modify: `frontend/src/components/Sidebar.tsx`

- [ ] **Step 1: Add `loading="lazy"` to the two social icon `<img>` tags**

In `frontend/src/components/Sidebar.tsx`, find the two social-icon lines (around lines 33 and 40 — they look like `<img className="icon" src="/linkedin.svg" alt="linkedin" />` and `<img className="icon" src="/github.svg" alt="github" />`).

Add `loading="lazy"` to each. After:

```tsx
<img className="icon" src="/linkedin.svg" alt="linkedin" loading="lazy" />
```

and

```tsx
<img className="icon" src="/github.svg" alt="github" loading="lazy" />
```

Do NOT add `loading="lazy"` to the avatar (`<img … className="app-header-avatar" src="/mees.png" …>` around line 20) — it's above the fold and should load eagerly.

- [ ] **Step 2: Verify build**

```bash
bash -c '
export NVM_DIR="$HOME/.nvm"
source "$NVM_DIR/nvm.sh"
nvm use 20 > /dev/null
cd /home/mees/git/mees.space/.worktrees/polish-bundle/frontend
npm run build 2>&1 | tail -8
'
```
Expected: build succeeds.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/components/Sidebar.tsx
git commit -m "feat(frontend): lazy-load sidebar social icons"
```

---

## Done Criteria

All three tasks committed. `go test ./...` passes (incl. the two new tests in `cmd/server`). `npm run build` succeeds. Manual verification from the spec's Verification section remains for the controller.
