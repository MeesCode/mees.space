# Polish Bundle — Design

**Date:** 2026-04-23
**Status:** Approved (auto-chain: design → plan → execute)
**Scope:** Two small polish items.

## Goal

1. **Proper HTTP 404 status** for content paths that don't resolve to a real page. The existing friendly in-page 404 UX is preserved; only the HTTP status code changes.
2. **Lazy loading on below-the-fold images** via native `loading="lazy"` attribute on `<img>` tags rendered from markdown and on the two social icons in the sidebar.

## Context

- The earlier SEO-bundle implementation intentionally left a soft-404 behavior in place: `serveContentPage` in `cmd/server/main.go` writes the shell with implicit HTTP 200 when `pagesSvc.GetPage` returns `ErrNotFound` or `ErrInvalidPath`. The client-side 404 UX runs correctly, but crawlers see 200 OK and may index phantom URLs.
- The frontend uses plain `<img>` tags. Markdown content renders through `react-markdown`, which emits `<img>` elements with no `loading` hint by default. The `Sidebar.tsx` component has one avatar image (above the fold) and two social icons.
- No Next.js `<Image>` component use; the site is a static export.

## Out of Scope

- Dark mode / light theme (intentionally dropped — the site's identity is its dark terminal look).
- Migrating to Next.js `<Image>` — the plumbing cost is not justified for a handful of images on a static-export site.
- Changing the existing "coffee break" 404 copy in `ContentPage.tsx`.
- Any other polish items.

## Architecture

Two independent changes:

| Component | Change |
|-----------|--------|
| `cmd/server/main.go` (`serveContentPage`) | Set HTTP 404 before writing body on page lookup failure; inject a no-index, generic-title shell |
| `cmd/server/main_test.go` (new) | `httptest` coverage for the 200 (published page) and 404 (missing page) branches |
| `frontend/src/components/MarkdownRenderer.tsx` | Add `components={{ img: … }}` override that forwards attributes + adds `loading="lazy"` and `decoding="async"` |
| `frontend/src/components/Sidebar.tsx` | Add `loading="lazy"` to the two social icon `<img>` tags |

## Backend — Proper HTTP 404

Current code in `cmd/server/main.go`:

```go
func serveContentPage(w http.ResponseWriter, r *http.Request, pagePath, baseURL string, pagesSvc *pages.Service, injector *seo.Injector) {
    if pagePath == "" {
        pagePath = "home"
    }
    page, err := pagesSvc.GetPage(pagePath)
    if err != nil {
        writeHTML(w, injector.Raw())
        return
    }
    // ...
}
```

Change the error branch to set 404 status and inject a generic no-index shell:

```go
func serveContentPage(w http.ResponseWriter, r *http.Request, pagePath, baseURL string, pagesSvc *pages.Service, injector *seo.Injector) {
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
        w.Write(injector.Inject(meta))
        return
    }
    // ... (unchanged)
}
```

Notes:
- Keeps injecting the shell so the client-side 404 UX ("This page is on a coffee break…") still runs for human visitors.
- `NoIndex: true` + generic title stops crawlers from indexing the phantom URL and doesn't leak any real page titles.
- `writeHTML` is not reused because we need to set the status code *before* writing the body; inlining the three header/status/write lines is clearer than adding a `writeHTMLStatus` variant for one caller.

The existing `writeHTML` helper stays unchanged for admin/raw paths.

## Backend — Test coverage

Add `cmd/server/main_test.go` with two table-driven test cases:

1. GET `/home-that-exists` on a DB with a `home` page → status 200, body contains the page's title injected into `<head>`.
2. GET `/does-not-exist` on a DB without that path → status 404, body contains the generic "Not Found" title and `<meta name="robots" content="noindex">`.

Uses `httptest.NewRecorder` + `httptest.NewRequest`. Reuses the existing test helpers where possible; otherwise builds a minimal `pages.Service` + `seo.Injector` fixture inline. Writes a fixture `dist/index.html` to a tmpdir for the injector to read.

## Frontend — Lazy images in MarkdownRenderer

Add a `components` prop to `<ReactMarkdown>` overriding the `img` element:

```tsx
<ReactMarkdown
  remarkPlugins={[remarkGfm]}
  rehypePlugins={[rehypeRaw, rehypeHighlight]}
  components={{
    img: (props) => <img {...props} loading="lazy" decoding="async" />,
  }}
>
  {content}
</ReactMarkdown>
```

- Forwards all react-markdown-provided props (src, alt, etc.) and adds two attributes.
- Native `loading="lazy"` is baseline-supported in every modern browser. `decoding="async"` is a small additional win on image-heavy pages.
- No TypeScript type plumbing needed — `props` inherits react-markdown's inferred `img` props.

## Frontend — Sidebar social icons

In `Sidebar.tsx`, add `loading="lazy"` to the two social icon `<img>` tags:

```tsx
<img className="icon" src="/linkedin.svg" alt="linkedin" loading="lazy" />
<img className="icon" src="/github.svg" alt="github" loading="lazy" />
```

Leave the avatar (`<img className="app-header-avatar" src="/mees.png" … />`) **eager** — it's above the fold and carries the first meaningful paint; lazy loading it would hurt perceived performance.

## Verification

- `go test ./...` passes, including the two new `cmd/server` tests.
- `npm run build` succeeds with no TypeScript errors.
- Manual: `curl -i http://localhost:8080/definitely-not-a-real-page` returns `HTTP/1.1 404 Not Found` with the injected shell body containing `<meta name="robots" content="noindex">`.
- Manual: load a content page with an image in a browser, open devtools Network, confirm the markdown image's request has `Priority: Low` and is deferred until scrolled near.

## Risks & Non-Concerns

- **Client-side 404 UX:** `ContentPage.tsx`'s error handler fetches `/api/pages/{path}` and gets a JSON 404, which it already handles by rendering the "coffee break" copy. The HTTP status on the shell doesn't affect that path. Tested mentally — no change in user-visible behavior beyond the status code.
- **Home page self-heal:** `pages.Service.GetPage("home")` self-heals if the DB row is missing but the file exists. So the 404 branch only fires for paths where *neither* exists, which is the correct condition.
- **Native lazy loading on small icons:** `loading="lazy"` on a 2KB SVG is technically noise (the request is cheap), but the attribute is harmless and keeps behavior consistent across the component. Not worth skipping.
