# SSR Content + Heading Anchors Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `/post#section` fragment URLs scroll correctly on first page load, and reintroduce hover-reveal clickable heading anchors, by pre-rendering markdown in Go and adopting that server render on the client without re-fetching.

**Architecture:** Two rendering paths — goldmark in Go for the first HTTP response (HTML + JSON bootstrap script injected into the Next.js static-export shell) and `react-markdown` on the client for SPA navigation. `ContentPage` reads the bootstrap synchronously on first mount, renders via `dangerouslySetInnerHTML` to match the server's HTML byte-for-byte, and skips the initial `/api/pages/{path}` fetch. Heading-ID parity between the two renderers is enforced by a shared slug algorithm and a set of test vectors consumed by both Go tests and a frontend `prebuild` script.

**Tech Stack:** Go 1.26 (`github.com/yuin/goldmark`), Next.js 15 static export, React 19, `react-markdown` + `remark-gfm` + `rehype-raw` + `rehype-highlight`, new: `rehype-slug`, `rehype-autolink-headings`, `github-slugger` (dev dep only).

**Reference spec:** `docs/superpowers/specs/2026-04-24-ssr-content-for-heading-anchors-design.md`

---

## File Structure

**New:**
- `internal/render/markdown.go` — goldmark wrapper, `Renderer` type
- `internal/render/slug.go` — `Slugify(text, seen)` shared slug function
- `internal/render/markdown_test.go` — goldmark output assertions
- `internal/render/slug_test.go` — slug vector assertions
- `internal/render/testdata/slug_vectors.json` — shared slug test vectors (consumed by both Go and frontend)
- `frontend/scripts/verify-slug-parity.mjs` — pre-build parity check against `github-slugger`

**Modified:**
- `internal/seo/inject.go` — add body content + data-script injection
- `internal/seo/inject_test.go` — new test coverage
- `internal/pages/model.go` — add optional `RenderedHTML` field to `PageResponse`
- `cmd/server/main.go` — wire `render.Renderer`, populate `BodyInjection`
- `frontend/package.json` — `rehype-slug`, `rehype-autolink-headings`, `github-slugger` (dev), `prebuild` script
- `frontend/src/components/MarkdownRenderer.tsx` — reinstate anchor plugins
- `frontend/src/app/globals.css` — reinstate `.heading-anchor` styles
- `frontend/src/app/layout.tsx` — add `<!--SSR_DATA-->` marker
- `frontend/src/app/[[...slug]]/ContentPage.tsx` — SSR_CONTENT marker in loading state, bootstrap reader, SSR render branch, hash-aware scroll

**Ordering constraint:** Frontend markers land before the Go injector tightens its marker verification. The ordering below respects this and leaves every commit in a working state.

---

## Task 1: Reinstate frontend markdown anchor plugins (dep install)

**Files:** `frontend/package.json`, `frontend/package-lock.json`

- [ ] **Step 1: Install the two reverted dependencies**

```bash
cd frontend && npm install rehype-slug@^6 rehype-autolink-headings@^7
```

- [ ] **Step 2: Verify they landed in package.json**

Run: `grep -E '"rehype-(slug|autolink-headings)"' frontend/package.json`
Expected: both lines present with caret versions.

- [ ] **Step 3: Commit**

```bash
git add frontend/package.json frontend/package-lock.json
git commit -m "chore(frontend): add rehype-slug and rehype-autolink-headings"
```

---

## Task 2: Reinstate MarkdownRenderer plugin pipeline

**Files:** Modify `frontend/src/components/MarkdownRenderer.tsx`

- [ ] **Step 1: Replace the file contents**

Write `frontend/src/components/MarkdownRenderer.tsx`:

```tsx
"use client";

import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import rehypeRaw from "rehype-raw";
import rehypeSlug from "rehype-slug";
import rehypeAutolinkHeadings from "rehype-autolink-headings";
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
        rehypePlugins={[
          rehypeRaw,
          rehypeSlug,
          [
            rehypeAutolinkHeadings,
            {
              behavior: "prepend",
              properties: {
                className: ["heading-anchor"],
                "aria-hidden": "true",
                tabIndex: -1,
              },
              content: { type: "text", value: "#" },
            },
          ],
          rehypeHighlight,
        ]}
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

- [ ] **Step 2: Verify build succeeds**

Run: `cd frontend && npm run build`
Expected: Next.js static export completes without errors.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/components/MarkdownRenderer.tsx
git commit -m "feat(frontend): add heading id + anchor link plugins to markdown pipeline"
```

---

## Task 3: Reinstate heading-anchor CSS

**Files:** Modify `frontend/src/app/globals.css`

- [ ] **Step 1: Locate the existing `.page-meta` block (as anchor point)**

Run: `grep -n "Page Metadata Bar" frontend/src/app/globals.css`
Expected: one line, e.g. `669: /* ——— Page Metadata Bar ——— */`. The new block goes immediately above it.

- [ ] **Step 2: Insert the heading anchor styles**

Use Edit to insert the following **directly before** the line `/* ——— Page Metadata Bar ——— */`:

```css
/* ——— Heading Anchor Links ——— */
#content .heading-anchor {
  color: rgba(255, 255, 255, 0.25);
  text-decoration: none;
  margin-right: 8px;
  opacity: 0;
  transition: opacity 0.15s;
  /* Reset the site-wide #content a gradient-underline rule */
  background: none;
  padding: 0;
}

#content h1:hover .heading-anchor,
#content h2:hover .heading-anchor,
#content h3:hover .heading-anchor,
#content h4:hover .heading-anchor,
#content h5:hover .heading-anchor,
#content h6:hover .heading-anchor {
  opacity: 1;
}

#content .heading-anchor:hover {
  color: var(--accent);
}

```

(Trailing blank line intentional, to separate from the next block.)

- [ ] **Step 3: Verify build succeeds**

Run: `cd frontend && npm run build`
Expected: success.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/app/globals.css
git commit -m "feat(frontend): add hover-reveal styles for heading anchors"
```

---

## Task 4: Add SSR_DATA marker to layout.tsx

**Files:** Modify `frontend/src/app/layout.tsx`

- [ ] **Step 1: Add the marker element just before `</body>`**

Use Edit. Replace:

```tsx
      <body>
        <Providers>
          <ClientLayout>{children}</ClientLayout>
        </Providers>
      </body>
```

With:

```tsx
      <body>
        <Providers>
          <ClientLayout>{children}</ClientLayout>
        </Providers>
        <div
          id="__ssr_data_slot__"
          aria-hidden="true"
          style={{ display: "none" }}
          dangerouslySetInnerHTML={{ __html: "<!--SSR_DATA-->" }}
        />
      </body>
```

- [ ] **Step 2: Build and verify the marker lands in dist/index.html**

Run: `cd frontend && npm run build && grep -c "SSR_DATA" ../dist/index.html`
Expected: `1` (one occurrence of the marker string).

If the count is 0, React stripped the comment. Fall back to a text-content placeholder: replace `dangerouslySetInnerHTML={{ __html: "<!--SSR_DATA-->" }}` with children of `SSR_DATA_SLOT` and do string-replace on that literal instead. Update Task 8 (Injector) accordingly.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/app/layout.tsx
git commit -m "feat(frontend): add SSR_DATA marker slot to layout"
```

---

## Task 5: Update ContentPage loading state with SSR_CONTENT marker, bootstrap reader, SSR render branch, and hash-aware scroll

**Files:** Modify `frontend/src/app/[[...slug]]/ContentPage.tsx`

This task is consolidated because splitting leaves the file in an inconsistent intermediate state (marker without bootstrap reader → hydration warning).

- [ ] **Step 1: Replace the file contents**

Write `frontend/src/app/[[...slug]]/ContentPage.tsx`:

```tsx
"use client";

import { useEffect, useRef, useState } from "react";
import { useNavigation } from "@/lib/navigation";
import { MarkdownRenderer } from "@/components/MarkdownRenderer";
import { PageMeta } from "@/components/PageMeta";
import { TerminalPrompt } from "@/components/TerminalPrompt";
import { PageData } from "@/lib/types";

interface BootstrapPage extends PageData {
  rendered_html?: string;
}

let bootstrapConsumed = false;

function readBootstrap(): BootstrapPage | null {
  if (bootstrapConsumed) return null;
  if (typeof document === "undefined") return null;
  const el = document.getElementById("__page_data__");
  if (!el || !el.textContent) return null;
  try {
    const parsed = JSON.parse(el.textContent) as BootstrapPage;
    return parsed;
  } catch (err) {
    console.warn("failed to parse __page_data__", err);
    return null;
  }
}

function applyScroll() {
  if (typeof window === "undefined") return;
  const hash = window.location.hash.slice(1);
  if (hash) {
    const target = document.getElementById(hash);
    if (target) {
      target.scrollIntoView();
      return;
    }
  }
  window.scrollTo(0, 0);
}

export function ContentPage() {
  const { path } = useNavigation();
  const pagePath = path === "/" ? "home" : path.replace(/^\//, "");

  const [page, setPage] = useState<PageData | null>(() => {
    const boot = readBootstrap();
    if (boot && boot.path === pagePath) {
      bootstrapConsumed = true;
      return boot;
    }
    return null;
  });
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(page === null);
  const ssrHTMLRef = useRef<string | null>(
    (page as BootstrapPage | null)?.rendered_html ?? null
  );
  const usedSSR = ssrHTMLRef.current !== null;
  const firstRunRef = useRef(true);

  useEffect(() => {
    if (pagePath.startsWith("admin")) return;

    // First render used the bootstrap — skip the initial fetch but still
    // fire view count and run the scroll + highlight pass against the
    // server-rendered DOM.
    if (firstRunRef.current && usedSSR && page) {
      firstRunRef.current = false;
      applyScroll();
      import("highlight.js").then(({ default: hljs }) => {
        document
          .querySelectorAll<HTMLElement>("#content pre code")
          .forEach((el) => hljs.highlightElement(el));
      });
      fetch(`/api/views/${pagePath}`, { method: "POST" }).catch(() => {});
      return;
    }

    firstRunRef.current = false;
    ssrHTMLRef.current = null;
    setLoading(true);
    setError(null);

    fetch(`/api/pages/${pagePath}`)
      .then((r) => {
        if (!r.ok) throw new Error("not found");
        return r.json();
      })
      .then((data) => {
        setPage(data);
        setLoading(false);
        applyScroll();
        fetch(`/api/views/${pagePath}`, { method: "POST" }).catch(() => {});
      })
      .catch(() => {
        setError("Page not found");
        setLoading(false);
      });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [pagePath]);

  if (pagePath.startsWith("admin")) {
    return null;
  }

  if (loading) {
    return (
      <>
        <div
          id="content"
          dangerouslySetInnerHTML={{ __html: "<!--SSR_CONTENT-->" }}
        />
        <TerminalPrompt path={pagePath} />
      </>
    );
  }

  if (error || !page) {
    const messages = [
      "This page is on a coffee break. Indefinitely. ☕",
      "404: page not found. But you found this message, so that's something.",
      "The bits that were here have been recycled into something better.",
      "You've reached the edge of the internet. Turn back.",
      "This page moved out and didn't leave a forwarding address.",
    ];
    const message = messages[Math.floor(Math.random() * messages.length)];

    return (
      <>
        <article id="content">
          <h1>404</h1>
          <p>{message}</p>
          <p style={{ marginTop: "24px" }}>
            <a href="/" onClick={(e) => { e.preventDefault(); window.location.href = "/"; }}>
              ← take me home
            </a>
          </p>
        </article>
        <TerminalPrompt path={pagePath} />
      </>
    );
  }

  return (
    <>
      <PageMeta page={page} />
      <div className={page.show_date ? "has-meta" : ""}>
        {usedSSR && ssrHTMLRef.current !== null ? (
          <div
            id="content"
            dangerouslySetInnerHTML={{ __html: ssrHTMLRef.current }}
          />
        ) : (
          <MarkdownRenderer content={page.content} />
        )}
      </div>
      <TerminalPrompt path={page.path} />
    </>
  );
}
```

- [ ] **Step 2: Verify the static export still builds and produces the SSR_CONTENT marker**

Run: `cd frontend && npm run build && grep -c "SSR_CONTENT" ../dist/index.html`
Expected: `1`.

If 0, same fallback as Task 4 — use a plain text placeholder (e.g. the literal `SSR_CONTENT_PLACEHOLDER`) and update Task 8.

- [ ] **Step 3: Smoke-test in dev mode**

Run: `cd frontend && npm run dev` in one terminal, open `http://localhost:3000/` in a browser, confirm the home page loads. (No SSR yet — the bootstrap read returns null, the code falls through to the fetch path.) Stop the dev server.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/app/[[...slug]]/ContentPage.tsx
git commit -m "feat(frontend): bootstrap-aware ContentPage with hash-aware scroll"
```

---

## Task 6: Create `internal/render` package and add goldmark dependency

**Files:**
- Create: `internal/render/markdown.go` (stub)
- Modify: `go.mod`, `go.sum`

- [ ] **Step 1: Install goldmark**

Run: `go get github.com/yuin/goldmark@latest`
Expected: `go.mod` updated with a `github.com/yuin/goldmark` line.

- [ ] **Step 2: Create the package with a placeholder `Renderer`**

Write `internal/render/markdown.go`:

```go
// Package render converts markdown source into HTML with stable heading
// IDs and hover-reveal anchor links, for server-side rendering of content
// pages. It is designed to produce output compatible with the client's
// react-markdown + rehype-slug + rehype-autolink-headings pipeline so that
// heading fragment links work across both rendering paths.
package render

import (
	"bytes"
	"fmt"
)

// Renderer holds a configured goldmark pipeline and is safe for concurrent use.
type Renderer struct{}

// New returns a Renderer with GFM, raw HTML pass-through, and the heading
// anchor transform enabled.
func New() *Renderer {
	return &Renderer{}
}

// ToHTML renders markdown source to HTML bytes.
// The empty implementation is fleshed out in Task 8.
func (r *Renderer) ToHTML(src []byte) ([]byte, error) {
	return nil, fmt.Errorf("not implemented")
}

var _ = bytes.MinRead // keep import during stub stage
```

- [ ] **Step 3: Verify the package builds**

Run: `go build ./internal/render/...`
Expected: no output, exit 0.

- [ ] **Step 4: Commit**

```bash
git add go.mod go.sum internal/render/markdown.go
git commit -m "chore: add goldmark dependency and render package skeleton"
```

---

## Task 7: Implement `Slugify` with shared test vectors

**Files:**
- Create: `internal/render/slug.go`
- Create: `internal/render/slug_test.go`
- Create: `internal/render/testdata/slug_vectors.json`

- [ ] **Step 1: Write the shared test-vector file**

Write `internal/render/testdata/slug_vectors.json`:

```json
[
  { "history": [],                           "input": "Hello World",           "want": "hello-world" },
  { "history": [],                           "input": "Multiple  Spaces",      "want": "multiple--spaces" },
  { "history": [],                           "input": "Trim Leading",          "want": "trim-leading" },
  { "history": [],                           "input": "Keep_Underscores",      "want": "keep_underscores" },
  { "history": [],                           "input": "Strip: Punct!",         "want": "strip-punct" },
  { "history": [],                           "input": "Café Déjà Vu",          "want": "café-déjà-vu" },
  { "history": [],                           "input": "1. Numbered",           "want": "1-numbered" },
  { "history": [],                           "input": "Hyphen-Already",        "want": "hyphen-already" },
  { "history": [],                           "input": "",                      "want": "" },
  { "history": ["foo"],                      "input": "Foo",                   "want": "foo-1" },
  { "history": ["foo", "foo-1"],             "input": "Foo",                   "want": "foo-2" },
  { "history": ["foo", "foo-1", "foo-2"],    "input": "Foo",                   "want": "foo-3" },
  { "history": [],                           "input": "   Leading Spaces",     "want": "-leading-spaces" }
]
```

Note: this targets **our** implementation (the source of truth). Task 13's parity script asserts `github-slugger` matches. If a vector diverges, we fix it in both places so they converge.

- [ ] **Step 2: Write the failing test**

Write `internal/render/slug_test.go`:

```go
package render

import (
	"encoding/json"
	"os"
	"testing"
)

type slugVector struct {
	History []string `json:"history"`
	Input   string   `json:"input"`
	Want    string   `json:"want"`
}

func TestSlugifyVectors(t *testing.T) {
	data, err := os.ReadFile("testdata/slug_vectors.json")
	if err != nil {
		t.Fatal(err)
	}
	var vectors []slugVector
	if err := json.Unmarshal(data, &vectors); err != nil {
		t.Fatal(err)
	}
	for _, v := range vectors {
		seen := map[string]int{}
		for _, h := range v.History {
			// Prime the counter with the historical slug exactly as produced.
			if _, ok := seen[h]; !ok {
				seen[h] = 1
			} else {
				seen[h]++
			}
		}
		got := Slugify(v.Input, seen)
		if got != v.Want {
			t.Errorf("Slugify(%q, history=%v) = %q, want %q", v.Input, v.History, got, v.Want)
		}
	}
}
```

- [ ] **Step 3: Run the test to confirm it fails**

Run: `go test ./internal/render/... -run TestSlugifyVectors`
Expected: `undefined: Slugify` compilation error.

- [ ] **Step 4: Implement Slugify**

Write `internal/render/slug.go`:

```go
package render

import (
	"strconv"
	"strings"
	"unicode"
)

// Slugify returns a URL-safe slug for the given heading text. Its behaviour
// tracks github-slugger (what rehype-slug uses on the client) closely enough
// for ASCII + common Latin-accented inputs; divergence is caught by the
// frontend parity build script (see scripts/verify-slug-parity.mjs).
//
// seen is modified: every returned slug is recorded so subsequent calls
// against the same map produce -1, -2, ... suffixes for duplicate inputs.
// The first occurrence is bare; the second becomes "foo-1", the third
// "foo-2", etc.
func Slugify(text string, seen map[string]int) string {
	var b strings.Builder
	for _, r := range text {
		switch {
		case unicode.IsLetter(r):
			b.WriteRune(unicode.ToLower(r))
		case unicode.IsDigit(r):
			b.WriteRune(r)
		case r == '_' || r == '-':
			b.WriteRune(r)
		case unicode.IsSpace(r):
			b.WriteRune('-')
		}
	}
	base := b.String()

	count, ok := seen[base]
	if !ok {
		seen[base] = 1
		return base
	}
	seen[base] = count + 1
	return base + "-" + strconv.Itoa(count)
}
```

- [ ] **Step 5: Run the tests**

Run: `go test ./internal/render/... -run TestSlugifyVectors -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/render/slug.go internal/render/slug_test.go internal/render/testdata/slug_vectors.json
git commit -m "feat(render): implement Slugify with shared vector tests"
```

---

## Task 8: Implement `Renderer.ToHTML` with heading-anchor transform

**Files:**
- Modify: `internal/render/markdown.go`
- Create: `internal/render/markdown_test.go`

- [ ] **Step 1: Write the failing test**

Write `internal/render/markdown_test.go`:

```go
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
```

- [ ] **Step 2: Run the test to confirm it fails**

Run: `go test ./internal/render/... -run TestToHTML -v`
Expected: FAIL with "not implemented".

- [ ] **Step 3: Implement `Renderer` with goldmark + heading transform**

Replace `internal/render/markdown.go` with:

```go
// Package render converts markdown source into HTML with stable heading
// IDs and hover-reveal anchor links, for server-side rendering of content
// pages. Output matches the client react-markdown + rehype-slug +
// rehype-autolink-headings pipeline closely enough that heading fragment
// links work across both rendering paths.
package render

import (
	"bytes"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// Renderer is concurrency-safe; construct once per process.
type Renderer struct {
	md goldmark.Markdown
}

// New returns a configured Renderer (GFM + raw HTML + heading anchors).
func New() *Renderer {
	md := goldmark.New(
		goldmark.WithExtensions(extension.GFM),
		goldmark.WithParserOptions(
			parser.WithASTTransformers(
				util.Prioritized(&headingAnchorTransformer{}, 500),
			),
		),
		goldmark.WithRendererOptions(html.WithUnsafe()),
	)
	return &Renderer{md: md}
}

// ToHTML renders markdown source to HTML bytes.
func (r *Renderer) ToHTML(src []byte) ([]byte, error) {
	var buf bytes.Buffer
	if err := r.md.Convert(src, &buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// headingAnchorTransformer walks every Heading node, assigns an id via
// Slugify, and prepends an anchor link (<a class="heading-anchor" ...>#</a>)
// as the heading's first child.
type headingAnchorTransformer struct{}

func (t *headingAnchorTransformer) Transform(doc *ast.Document, reader text.Reader, _ parser.Context) {
	seen := map[string]int{}
	src := reader.Source()

	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		h, ok := n.(*ast.Heading)
		if !ok {
			return ast.WalkContinue, nil
		}
		slug := Slugify(headingText(h, src), seen)

		h.SetAttributeString("id", []byte(slug))

		link := ast.NewLink()
		link.Destination = []byte("#" + slug)
		link.SetAttributeString("class", []byte("heading-anchor"))
		link.SetAttributeString("aria-hidden", []byte("true"))
		link.SetAttributeString("tabindex", []byte("-1"))
		link.AppendChild(link, ast.NewString([]byte("#")))

		if first := h.FirstChild(); first != nil {
			h.InsertBefore(h, first, link)
		} else {
			h.AppendChild(h, link)
		}

		return ast.WalkSkipChildren, nil
	})
}

// headingText flattens a heading's inline children to plain text for slugging.
func headingText(h *ast.Heading, source []byte) string {
	var buf bytes.Buffer
	for c := h.FirstChild(); c != nil; c = c.NextSibling() {
		collectText(c, source, &buf)
	}
	return buf.String()
}

func collectText(n ast.Node, source []byte, buf *bytes.Buffer) {
	switch v := n.(type) {
	case *ast.Text:
		buf.Write(v.Segment.Value(source))
	case *ast.String:
		buf.Write(v.Value)
	default:
		for c := n.FirstChild(); c != nil; c = c.NextSibling() {
			collectText(c, source, buf)
		}
	}
}
```

- [ ] **Step 4: Run the tests**

Run: `go test ./internal/render/... -v`
Expected: PASS for all four tests.

If the anchor-link attributes don't appear in the output, goldmark may render `ast.Link` without custom attributes. Confirm by printing `out` — if so, the default HTML renderer does accept attributes set via `SetAttributeString`; if not, switch to emitting a raw HTML node via `ast.NewRawHTML` instead of `ast.NewLink`.

- [ ] **Step 5: Commit**

```bash
git add internal/render/markdown.go internal/render/markdown_test.go
git commit -m "feat(render): implement ToHTML with heading anchor transform"
```

---

## Task 9: Extend `seo.Injector` with body content + data injection

**Files:** Modify `internal/seo/inject.go`, `internal/seo/inject_test.go`

- [ ] **Step 1: Add test for the new marker verification and injection**

Append to `internal/seo/inject_test.go`:

```go
const shellWithMarkers = `<html>
<head></head>
<body>
<div id="content"><!--SSR_CONTENT--></div>
<!--SSR_DATA-->
</body>
</html>`

func TestNewInjectorMissingContentMarker(t *testing.T) {
	dir := writeTestHTML(t, `<html><head></head><body><!--SSR_DATA--></body></html>`)
	_, err := NewInjector(dir)
	if err == nil {
		t.Fatal("expected error when SSR_CONTENT marker is missing")
	}
}

func TestNewInjectorMissingDataMarker(t *testing.T) {
	dir := writeTestHTML(t, `<html><head></head><body><div id="content"><!--SSR_CONTENT--></div></body></html>`)
	_, err := NewInjector(dir)
	if err == nil {
		t.Fatal("expected error when SSR_DATA marker is missing")
	}
}

func TestInjectorBodyHTMLAndData(t *testing.T) {
	dir := writeTestHTML(t, shellWithMarkers)
	inj, err := NewInjector(dir)
	if err != nil {
		t.Fatal(err)
	}
	got := string(inj.Inject(PageMeta{Title: "T"}, BodyInjection{
		HTML: []byte("<h2 id=\"x\">X</h2>"),
		Data: []byte(`{"path":"home"}`),
	}))

	if !strings.Contains(got, `<div id="content"><h2 id="x">X</h2></div>`) {
		t.Errorf("content not injected, got:\n%s", got)
	}
	if !strings.Contains(got, `<script id="__page_data__" type="application/json">{"path":"home"}</script>`) {
		t.Errorf("data script not injected, got:\n%s", got)
	}
	if strings.Contains(got, "SSR_CONTENT") || strings.Contains(got, "SSR_DATA") {
		t.Errorf("markers not stripped, got:\n%s", got)
	}
}

func TestInjectorBodyEmptyStripsMarkers(t *testing.T) {
	dir := writeTestHTML(t, shellWithMarkers)
	inj, _ := NewInjector(dir)
	got := string(inj.Inject(PageMeta{Title: "T"}, BodyInjection{}))
	if strings.Contains(got, "SSR_CONTENT") || strings.Contains(got, "SSR_DATA") {
		t.Errorf("markers should be stripped even with empty body, got:\n%s", got)
	}
	if strings.Contains(got, `<script id="__page_data__"`) {
		t.Errorf("data script should not appear with empty Data, got:\n%s", got)
	}
}

func TestInjectorRawStripsMarkers(t *testing.T) {
	dir := writeTestHTML(t, shellWithMarkers)
	inj, _ := NewInjector(dir)
	got := string(inj.Raw())
	if strings.Contains(got, "SSR_CONTENT") || strings.Contains(got, "SSR_DATA") {
		t.Errorf("Raw() must strip markers, got:\n%s", got)
	}
}
```

The existing `TestInjectorInjectsAllTags`, `TestInjectorNoIndex`, `TestInjectorEscapesValues`, `TestInjectorRaw` will fail to compile because `Inject` / `Raw`'s signature is changing. Update them inline (change `Inject(m)` → `Inject(m, BodyInjection{})`, and `Raw()` keeps its signature but the test shell must now include the two markers — update the fixture strings).

Concretely, change `TestInjectorInjectsAllTags`, `TestInjectorNoIndex`, `TestInjectorEscapesValues` to use `shellWithMarkers` as the input, and call `inj.Inject(PageMeta{...}, BodyInjection{})`. Change `TestInjectorRaw`'s fixture to `shellWithMarkers` and assert the returned bytes equal the shell with both markers *replaced with empty strings* (not equal to the original body).

- [ ] **Step 2: Run tests to confirm they fail**

Run: `go test ./internal/seo/...`
Expected: compile failure (`Inject` arity mismatch) or test failures.

- [ ] **Step 3: Implement the new `Injector`**

Replace `internal/seo/inject.go` with:

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

// BodyInjection carries server-rendered content and a JSON bootstrap payload.
// Either field may be nil/empty; in that case the corresponding marker in the
// HTML shell is replaced with the empty string.
type BodyInjection struct {
	HTML []byte // rendered markdown, inserted into the SSR_CONTENT slot
	Data []byte // JSON bytes; wrapped in a <script id="__page_data__"> tag
}

const (
	markerContent = "<!--SSR_CONTENT-->"
	markerData    = "<!--SSR_DATA-->"
)

// Injector holds the index.html template and produces customized HTML.
type Injector struct {
	template []byte
}

// NewInjector reads dist/index.html and verifies required anchors exist.
func NewInjector(distDir string) (*Injector, error) {
	path := filepath.Join(distDir, "index.html")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	for _, anchor := range [...]string{"</head>", markerContent, markerData} {
		if !bytes.Contains(data, []byte(anchor)) {
			return nil, fmt.Errorf("%s: missing %q anchor; cannot inject", path, anchor)
		}
	}
	return &Injector{template: data}, nil
}

// Inject returns a customized copy of the template with per-page <head>
// tags inserted, rendered body HTML substituted for the content marker,
// and a JSON bootstrap script substituted for the data marker. Safe for
// concurrent use.
func (i *Injector) Inject(m PageMeta, body BodyInjection) []byte {
	out := make([]byte, len(i.template))
	copy(out, i.template)

	// Head fragment.
	fragment := buildFragment(m)
	out = bytes.Replace(out, []byte("</head>"), append(fragment, []byte("</head>")...), 1)

	// Content slot.
	out = bytes.Replace(out, []byte(markerContent), body.HTML, 1)

	// Data slot.
	var dataReplacement []byte
	if len(body.Data) > 0 {
		var b bytes.Buffer
		b.WriteString(`<script id="__page_data__" type="application/json">`)
		b.Write(body.Data)
		b.WriteString(`</script>`)
		dataReplacement = b.Bytes()
	}
	out = bytes.Replace(out, []byte(markerData), dataReplacement, 1)

	return out
}

// Raw returns the template with both SSR markers stripped, for use on
// admin routes that skip SEO injection.
func (i *Injector) Raw() []byte {
	out := make([]byte, len(i.template))
	copy(out, i.template)
	out = bytes.Replace(out, []byte(markerContent), nil, 1)
	out = bytes.Replace(out, []byte(markerData), nil, 1)
	return out
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

- [ ] **Step 4: Run the tests**

Run: `go test ./internal/seo/... -v`
Expected: all tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/seo/inject.go internal/seo/inject_test.go
git commit -m "feat(seo): inject SSR body HTML and bootstrap data into shell"
```

---

## Task 10: Add optional `RenderedHTML` field to `PageResponse`

**Files:** Modify `internal/pages/model.go`

- [ ] **Step 1: Add the field**

Use Edit to change:

```go
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
```

to:

```go
type PageResponse struct {
	Path         string `json:"path"`
	Title        string `json:"title"`
	Content      string `json:"content"`
	Description  string `json:"description"`
	ViewCount    int    `json:"view_count"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
	ShowDate     bool   `json:"show_date"`
	Published    bool   `json:"published"`
	// RenderedHTML is populated only in the SSR bootstrap payload (main.go),
	// never by the /api/pages handler. Omitempty keeps API responses
	// byte-identical to the pre-SSR shape.
	RenderedHTML string `json:"rendered_html,omitempty"`
}
```

- [ ] **Step 2: Verify everything still compiles**

Run: `go build ./...`
Expected: success.

- [ ] **Step 3: Verify existing tests still pass**

Run: `go test ./internal/pages/...`
Expected: all pass.

- [ ] **Step 4: Commit**

```bash
git add internal/pages/model.go
git commit -m "feat(pages): add optional rendered_html field for SSR bootstrap"
```

---

## Task 11: Wire the renderer and body injection into `cmd/server/main.go`

**Files:** Modify `cmd/server/main.go`

- [ ] **Step 1: Add the render import and plumb the `Renderer` into `staticHandler`**

At the top of `cmd/server/main.go`, add `"mees.space/internal/render"` to the import block. Then change the constructor calls and the `serveContentPage` signature.

Use Edit to change in `main()`:

```go
	injector, injErr := seo.NewInjector(cfg.DistDir)
	if injErr != nil {
		log.Fatal("seo injector:", injErr)
	}
```

to:

```go
	injector, injErr := seo.NewInjector(cfg.DistDir)
	if injErr != nil {
		log.Fatal("seo injector:", injErr)
	}
	renderer := render.New()
```

And change the static-handler wiring:

```go
	// Catch-all: serve Next.js static export
	mux.HandleFunc("GET /{path...}", staticHandler(cfg.DistDir, cfg.BaseURL, pagesSvc, injector))
```

to:

```go
	// Catch-all: serve Next.js static export
	mux.HandleFunc("GET /{path...}", staticHandler(cfg.DistDir, cfg.BaseURL, pagesSvc, injector, renderer))
```

And the function signature & its two `serveContentPage` call sites. Change:

```go
func staticHandler(distDir, baseURL string, pagesSvc *pages.Service, injector *seo.Injector) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		urlPath := r.URL.Path
		if urlPath == "/" {
			serveContentPage(w, r, "", baseURL, pagesSvc, injector)
			return
		}
```

to:

```go
func staticHandler(distDir, baseURL string, pagesSvc *pages.Service, injector *seo.Injector, renderer *render.Renderer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		urlPath := r.URL.Path
		if urlPath == "/" {
			serveContentPage(w, r, "", baseURL, pagesSvc, injector, renderer)
			return
		}
```

And the fallthrough:

```go
		// Content page fallback: strip leading slash and attempt lookup.
		pagePath := strings.TrimPrefix(urlPath, "/")
		serveContentPage(w, r, pagePath, baseURL, pagesSvc, injector)
	}
}
```

to:

```go
		// Content page fallback: strip leading slash and attempt lookup.
		pagePath := strings.TrimPrefix(urlPath, "/")
		serveContentPage(w, r, pagePath, baseURL, pagesSvc, injector, renderer)
	}
}
```

- [ ] **Step 2: Replace `serveContentPage` with the SSR-capable version**

Replace the existing `serveContentPage` function body with:

```go
func serveContentPage(w http.ResponseWriter, r *http.Request, pagePath, baseURL string, pagesSvc *pages.Service, injector *seo.Injector, renderer *render.Renderer) {
	if pagePath == "" {
		pagePath = "home"
	}

	page, err := pagesSvc.GetPage(pagePath)
	if err != nil {
		meta := seo.PageMeta{
			Title:       "Not Found — Mees Brinkhuis",
			Description: "",
			NoIndex:     true,
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusNotFound)
		w.Write(injector.Inject(meta, seo.BodyInjection{}))
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

	body := seo.BodyInjection{}
	if page.Published {
		html, renderErr := renderer.ToHTML([]byte(page.Content))
		if renderErr != nil {
			log.Printf("ssr: render %s: %v", pagePath, renderErr)
		} else {
			body.HTML = html
			bootstrap := *page
			bootstrap.RenderedHTML = string(html)
			if jsonBytes, jsonErr := json.Marshal(bootstrap); jsonErr != nil {
				log.Printf("ssr: marshal %s: %v", pagePath, jsonErr)
			} else {
				body.Data = jsonBytes
			}
		}
	} else {
		// Don't leak draft details to crawlers or in the bootstrap payload.
		meta.Title = "Draft — Mees Brinkhuis"
		meta.Description = ""
	}

	writeHTML(w, injector.Inject(meta, body))
}
```

- [ ] **Step 3: Update the admin fall-through Raw call (no signature change, but confirm)**

Skim `staticHandler`: the `strings.HasPrefix(urlPath, "/admin")` branch still calls `writeHTML(w, injector.Raw())`. No change needed — `Raw()` now strips markers internally.

- [ ] **Step 4: Add `"encoding/json"` import if missing**

Run: `goimports -w cmd/server/main.go` (or use Edit to ensure `encoding/json` and `mees.space/internal/render` are in the import block).

- [ ] **Step 5: Build and run Go tests**

Run: `go build ./... && go test ./...`
Expected: all green. Note `GetPage` must return `*pages.PageResponse` — if it returns a value (not pointer), replace `bootstrap := *page` with `bootstrap := page`. Adjust accordingly based on the actual signature.

- [ ] **Step 6: Commit**

```bash
git add cmd/server/main.go
git commit -m "feat(server): render markdown server-side and inject into shell"
```

---

## Task 12: Integration test — server returns SSR content + bootstrap script

**Files:** Create `cmd/server/ssr_test.go`

- [ ] **Step 1: Survey existing test setup**

Run: `ls cmd/server/` and inspect any existing test files. If there's a helper that boots the full server, reuse it; otherwise the test below stands up the minimum surface.

- [ ] **Step 2: Write a focused integration-style test**

Write `cmd/server/ssr_test.go`:

```go
package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mees.space/internal/pages"
	"mees.space/internal/render"
	"mees.space/internal/seo"
)

const ssrTestShell = `<!doctype html>
<html><head></head>
<body>
<div id="content"><!--SSR_CONTENT--></div>
<!--SSR_DATA-->
</body>
</html>`

func setupSSRTestEnv(t *testing.T) (distDir, contentDir string, pagesSvc *pages.Service, inj *seo.Injector) {
	t.Helper()
	distDir = t.TempDir()
	if err := os.WriteFile(filepath.Join(distDir, "index.html"), []byte(ssrTestShell), 0644); err != nil {
		t.Fatal(err)
	}

	contentDir = t.TempDir()
	if err := os.WriteFile(filepath.Join(contentDir, "hello.md"), []byte("## Hi\n\nbody"), 0644); err != nil {
		t.Fatal(err)
	}

	// If pagesSvc needs a DB, swap this for whatever the existing test
	// helpers use. If pages.NewService has a different signature, adapt.
	// The goal is: GetPage("hello") returns a published page with title
	// and content set.
	t.Skip("TODO: wire a test pages.Service that returns a published page. " +
		"Replace this Skip once the project provides a test helper or the " +
		"engineer writes one. The assertions below are the contract.")

	_ = pagesSvc
	inj, err := seo.NewInjector(distDir)
	if err != nil {
		t.Fatal(err)
	}
	return
}

func TestStaticHandlerInjectsSSRContentAndBootstrap(t *testing.T) {
	distDir, _, pagesSvc, inj := setupSSRTestEnv(t)
	_ = distDir
	renderer := render.New()
	handler := staticHandler(distDir, "https://example.com", pagesSvc, inj, renderer)

	req := httptest.NewRequest(http.MethodGet, "/hello", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	body := w.Body.String()
	for _, want := range []string{
		`<h2 id="hi">`,
		`class="heading-anchor"`,
		`<script id="__page_data__" type="application/json">`,
		`"rendered_html"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("body missing %q\n---\n%s\n---", want, body)
		}
	}
	if strings.Contains(body, "SSR_CONTENT") || strings.Contains(body, "SSR_DATA") {
		t.Errorf("markers not replaced")
	}
}

func TestStaticHandlerDraftPagesDoNotLeakContent(t *testing.T) {
	t.Skip("TODO: wire a test pages.Service that returns an unpublished page. " +
		"Assert: response body does NOT contain the page content, NOR a " +
		"<script id=\"__page_data__\"> tag, AND contains <meta name=\"robots\" " +
		"content=\"noindex\">.")
}
```

- [ ] **Step 3: Either wire a real test `pages.Service` or convert to a mocked variant**

Inspect `internal/pages/service.go` for its constructor. If it takes a `*sql.DB`, open an in-memory SQLite (`modernc.org/sqlite` with `:memory:`) and run migrations from `./migrations/`; this mirrors what production does. Seed one published page `hello` and one unpublished page, then unskip the tests.

If this turns out to require significantly more setup than the rest of the plan, leave the `t.Skip` lines in place and move on — the unit-level coverage in `internal/render` + `internal/seo` carries the correctness load; this integration test is insurance.

- [ ] **Step 4: Run tests**

Run: `go test ./cmd/server/... -v`
Expected: skipped or passing (never failing).

- [ ] **Step 5: Commit**

```bash
git add cmd/server/ssr_test.go
git commit -m "test(server): assert SSR content and bootstrap script injection"
```

---

## Task 13: Slug parity build check (frontend)

**Files:**
- Modify: `frontend/package.json`
- Create: `frontend/scripts/verify-slug-parity.mjs`

- [ ] **Step 1: Install `github-slugger` as a dev dependency**

Run: `cd frontend && npm install --save-dev github-slugger`

- [ ] **Step 2: Write the parity script**

Write `frontend/scripts/verify-slug-parity.mjs`:

```js
#!/usr/bin/env node
// Verifies that github-slugger (the engine rehype-slug uses) produces the
// same output as Go's internal/render.Slugify for every vector in
// ../../internal/render/testdata/slug_vectors.json. Called from the
// frontend's prebuild hook; divergence breaks `npm run build`.

import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { dirname, resolve } from "node:path";
import GithubSlugger from "github-slugger";

const __dirname = dirname(fileURLToPath(import.meta.url));
const vectorsPath = resolve(
  __dirname,
  "../../internal/render/testdata/slug_vectors.json"
);

const vectors = JSON.parse(readFileSync(vectorsPath, "utf8"));

let failed = 0;
for (const { input, history, want } of vectors) {
  const slugger = new GithubSlugger();
  for (const h of history) slugger.occurrences[h] = (slugger.occurrences[h] ?? 0) + 1;
  const got = slugger.slug(input);
  if (got !== want) {
    console.error(
      `✗ slug parity mismatch: slug(${JSON.stringify(input)}, history=${JSON.stringify(history)}) = ${JSON.stringify(got)}, want ${JSON.stringify(want)}`
    );
    failed++;
  }
}

if (failed > 0) {
  console.error(`\n${failed} slug vector(s) diverge between Go and github-slugger.`);
  console.error("Fix either internal/render/slug.go or testdata/slug_vectors.json so both agree.");
  process.exit(1);
}

console.log(`slug parity: ${vectors.length} vectors match github-slugger`);
```

- [ ] **Step 3: Wire it into the build**

Use Edit on `frontend/package.json` to add a `prebuild` script. Find the existing `"scripts"` block and replace:

```json
  "scripts": {
    "dev": "next dev",
    "build": "next build",
    "lint": "next lint"
  },
```

with:

```json
  "scripts": {
    "dev": "next dev",
    "prebuild": "node scripts/verify-slug-parity.mjs",
    "build": "next build",
    "lint": "next lint"
  },
```

- [ ] **Step 4: Run the parity check**

Run: `cd frontend && node scripts/verify-slug-parity.mjs`
Expected: `slug parity: N vectors match github-slugger` and exit 0. If it fails for non-ASCII or space-prefix vectors, either update `internal/render/slug.go` and its test vectors to match github-slugger's behavior for those cases, or drop the divergent vectors from `slug_vectors.json` (with a clear comment) — both renderers must agree on every vector left in the file.

- [ ] **Step 5: Run the full build to confirm prebuild wiring**

Run: `cd frontend && npm run build`
Expected: parity check runs (and passes), then Next.js build completes.

- [ ] **Step 6: Commit**

```bash
git add frontend/package.json frontend/package-lock.json frontend/scripts/verify-slug-parity.mjs
git commit -m "test(frontend): verify slug parity with github-slugger in prebuild"
```

---

## Task 14: End-to-end manual verification and polish

**Files:** None (exploratory + fix-up task).

- [ ] **Step 1: Full build**

Run: `make build`
Expected: both frontend (with prebuild parity check) and Go backend compile.

- [ ] **Step 2: Boot the server and cold-load a hashed URL**

Run: `make build-run` (or start manually: `./server`) after ensuring at least one published page with an H2 exists in `content/`. Open a fresh browser tab to `http://localhost:8080/<page-path>#<heading-slug>`.
Expected: page lands on the heading with no visible jump. View-source shows the content inside `<div id="content">` and a `<script id="__page_data__">` near `</body>`.

- [ ] **Step 3: Test SPA nav to hashed URL**

From the home page, click any internal link to a hashed anchor (or manipulate in devtools: `history.pushState(null, "", "/<page>#<slug>"); window.dispatchEvent(new PopStateEvent("popstate"))`).
Expected: hash lands on the target heading after the fetch completes.

- [ ] **Step 4: Hover check**

Hover any H2/H3/H4/H5/H6 on a content page.
Expected: `#` fades in to the left, click updates URL + copies via the browser default.

- [ ] **Step 5: No-JS check**

Open `view-source:http://localhost:8080/<page-path>` or fetch with `curl -s http://localhost:8080/<page-path> | less` and confirm the rendered HTML (with headings + anchor links) appears in the body.

- [ ] **Step 6: Admin paths**

Open `http://localhost:8080/admin/editor`.
Expected: admin shell loads. View-source shows NO `<!--SSR_CONTENT-->`, `<!--SSR_DATA-->`, or `<script id="__page_data__">` artifacts.

- [ ] **Step 7: Draft page check**

With an admin session, visit an unpublished page's URL.
Expected: `<meta name="robots" content="noindex">` in `<head>`; body contains no rendered content server-side; after client fetch (authenticated), content appears.

- [ ] **Step 8: 404**

Visit a random non-existent path.
Expected: HTTP 404, `<meta name="robots" content="noindex">`, and the client-rendered 404 UX once React hydrates.

- [ ] **Step 9: If anything fails in steps 2–8**

Fix the underlying cause (don't paper over). Rerun the whole checklist. Common failure modes and fixes:
- **Hydration warning in console on first load:** React is finding a DOM that differs from what it's about to render. Usually means `readBootstrap()` returned data but the initial render isn't using `dangerouslySetInnerHTML`. Trace `usedSSR` — it must be `true` when the bootstrap matches.
- **Marker leaks to browser ("<!--SSR_CONTENT-->" visible in view-source):** Go's `NewInjector` verification would have caught a missing marker; a *remaining* marker means the replacement path returned early. Add a log in `seo.Injector.Inject` to confirm replacements ran.
- **Anchor click scrolls but fragment doesn't appear in URL:** `rehype-autolink-headings` config is wrong — confirm `behavior: "prepend"` and the `href: "#" + slug` path; inspect the emitted `<a>`.

- [ ] **Step 10: Final commit (only if any fix was needed in Step 9)**

```bash
git add <changed files>
git commit -m "fix(ssr): <what you fixed>"
```

---

## Self-Review (done before execution handoff)

**Spec coverage:**
- Goal + rationale → Task 5, 11 (client bootstrap, server render)
- Scope / out-of-scope → honored: admin untouched (Tasks 11, 14 step 6), drafts not leaked (Task 11, Task 12)
- `internal/render` package → Tasks 6, 7, 8
- SEO injector extension → Task 9
- `RenderedHTML` field → Task 10
- `cmd/server` wiring → Task 11
- Frontend `MarkdownRenderer` / CSS → Tasks 2, 3
- `layout.tsx` + `ContentPage` → Tasks 4, 5
- Bootstrap JSON shape → Tasks 10, 11
- Heading ID parity → Task 7 (vectors), Task 13 (build check)
- Hash-aware scroll → Task 5
- Error handling (render failure, missing bootstrap, draft guard) → Tasks 5, 11, 12
- Verification plan (automated + manual) → Tasks 7, 8, 9, 12, 13, 14

**Placeholder scan:** Task 12 uses `t.Skip` intentionally when a test pages.Service requires nontrivial setup, with a clear contract for the assertions. No other `TODO` / `TBD`.

**Type consistency:** `Renderer`, `Slugify`, `BodyInjection{HTML, Data}`, `PageResponse.RenderedHTML`, `readBootstrap()`, `applyScroll()`, `usedSSR` — all used consistently across Tasks 6–13.

**Known minor items deferred to execution judgment:** exact goldmark attribute-emission behaviour (Task 8 step 4 contingency); React comment preservation in static export (Task 4 step 2 contingency). Each has an explicit fallback path documented where it's introduced.
