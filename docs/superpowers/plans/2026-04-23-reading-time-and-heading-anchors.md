# Reading Time & Heading Anchors Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a metadata bar (date · reading time · views) above dated public pages, plus hover-reveal anchor links on all rendered markdown headings.

**Architecture:** Frontend-only change. Extend the existing `react-markdown` rehype plugin chain with `rehype-slug` + `rehype-autolink-headings` for anchor functionality. Add a new `PageMeta` React component conditionally rendered above `MarkdownRenderer` in `ContentPage` when the page's `show_date` flag is true. No backend changes, no database migrations, no new API fields.

**Tech Stack:** Next.js 15 (static export), React 19, TypeScript, `react-markdown`, `remark-gfm`, `rehype-raw`, `rehype-highlight`, new: `rehype-slug` + `rehype-autolink-headings`. Plain CSS in `globals.css`.

**Reference spec:** `docs/superpowers/specs/2026-04-23-reading-time-and-heading-anchors-design.md`

**Note on testing:** The frontend has no automated test harness (no vitest/jest config, no test scripts in `frontend/package.json`). The spec explicitly declines setting one up for this feature. Verification is via `npm run build` (type + build check) and manual browser checks listed per task.

---

## File Map

**Create:**
- `frontend/src/components/PageMeta.tsx` — the metadata bar component

**Modify:**
- `frontend/package.json` + `frontend/package-lock.json` — add two dependencies
- `frontend/src/components/MarkdownRenderer.tsx` — extend rehype plugin chain
- `frontend/src/app/[[...slug]]/ContentPage.tsx` — integrate `PageMeta`
- `frontend/src/app/globals.css` — styling for `.page-meta`, `.has-meta #content`, and `.heading-anchor`

Backend files and admin routes are not touched. The admin editor (`frontend/src/app/admin/editor/page.tsx`) uses `MarkdownRenderer` directly, so anchor links automatically propagate there with no code change — exactly what the spec calls for.

---

## Task 1: Add npm dependencies

**Files:**
- Modify: `frontend/package.json`
- Modify: `frontend/package-lock.json`

- [ ] **Step 1: Install the two rehype plugins**

Run from repo root:
```bash
cd frontend && npm install rehype-slug rehype-autolink-headings
```

Expected: both packages install cleanly and are added to `dependencies` in `package.json`. No peer-dep warnings for these packages with the existing `react-markdown@^10`.

- [ ] **Step 2: Verify package.json entries**

Run:
```bash
grep -E '"rehype-(slug|autolink-headings)"' frontend/package.json
```

Expected output (versions may differ):
```
    "rehype-autolink-headings": "^7.x.x",
    "rehype-slug": "^6.x.x",
```

- [ ] **Step 3: Commit**

```bash
git add frontend/package.json frontend/package-lock.json
git commit -m "chore: add rehype-slug and rehype-autolink-headings"
```

---

## Task 2: Extend MarkdownRenderer pipeline with anchor plugins

**Files:**
- Modify: `frontend/src/components/MarkdownRenderer.tsx` (rewrite — 22 lines total)

- [ ] **Step 1: Replace the full contents of `MarkdownRenderer.tsx`**

The current file uses `[rehypeRaw, rehypeHighlight]`. Replace the entire file with the updated pipeline:

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
      >
        {content}
      </ReactMarkdown>
    </div>
  );
}
```

Order matters: `rehypeRaw` → `rehypeSlug` (assigns heading IDs) → `rehypeAutolinkHeadings` (uses those IDs) → `rehypeHighlight` (operates on code blocks, independent).

- [ ] **Step 2: Verify the build compiles**

Run:
```bash
cd frontend && npm run build
```

Expected: build succeeds. No TypeScript errors. No "Module not found" errors. The static export completes and writes to `dist/`.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/components/MarkdownRenderer.tsx
git commit -m "feat(frontend): add heading IDs and anchor links to markdown pipeline"
```

---

## Task 3: Add CSS for heading anchor links

**Files:**
- Modify: `frontend/src/app/globals.css` (append to end of file)

- [ ] **Step 1: Append anchor link styles to `globals.css`**

Append to the end of `frontend/src/app/globals.css`:

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

The `background: none; padding: 0` reset is required because the existing `#content a` rule applies a teal gradient-underline and padding meant for prose links — without the reset, the `#` would inherit those styles.

- [ ] **Step 2: Manual browser verification**

Run:
```bash
cd frontend && npm run dev
```

Then in another terminal, start the backend so `/api/pages/...` is reachable:
```bash
make build-run
```
(Or whatever is already running — if the backend is already up, skip this.)

Open `http://localhost:3000/` (or whatever the dev port is — Next.js will print it) on a page that contains multiple headings at different levels.

Verify all of the following:
- Hovering over each heading (h1 through h6) reveals a `#` glyph before the heading text, in a muted color.
- Hovering the `#` itself turns it teal (the `--accent` color).
- Clicking the `#` updates the URL to include `#<slug>` and scrolls to the heading.
- Refreshing the page with a `#<slug>` fragment scrolls to the heading (after the client-side fetch completes — may take a beat).
- Two headings with identical text produce IDs like `my-heading` and `my-heading-1`.
- Admin editor preview (`/admin/editor`, logged in, viewing a draft) also shows hover-reveal `#` on headings (spec says anchor links apply everywhere `MarkdownRenderer` is used).

- [ ] **Step 3: Commit**

```bash
git add frontend/src/app/globals.css
git commit -m "feat(frontend): style heading anchor links with hover reveal"
```

---

## Task 4: Create the `PageMeta` component

**Files:**
- Create: `frontend/src/components/PageMeta.tsx`

- [ ] **Step 1: Create `PageMeta.tsx`**

Create `frontend/src/components/PageMeta.tsx` with this exact content:

```tsx
"use client";

import { PageData } from "@/lib/types";

interface Props {
  page: PageData;
}

export function PageMeta({ page }: Props) {
  if (!page.show_date) return null;

  const date = new Date(page.created_at).toLocaleDateString("en-US", {
    year: "numeric",
    month: "short",
    day: "numeric",
  });

  const words = page.content.trim().split(/\s+/).filter(Boolean).length;
  const minutes = Math.max(1, Math.ceil(words / 200));

  const viewLabel = page.view_count === 1 ? "1 view" : `${page.view_count} views`;

  return (
    <div className="page-meta">
      <span>{date}</span>
      <span className="sep">·</span>
      <span>{minutes} min read</span>
      <span className="sep">·</span>
      <span>{viewLabel}</span>
    </div>
  );
}
```

Notes for the implementer:
- `PageData` already imports cleanly from `@/lib/types` — no type changes needed (see `frontend/src/lib/types.ts`, existing `PageData` interface has all the fields used here: `show_date`, `created_at`, `content`, `view_count`).
- Reading time calc is intentionally inline (no helper function, no `useMemo`, no npm dep). The user explicitly requested this in brainstorming.
- The component returns `null` (not an empty div) when `show_date` is false so it renders nothing at all on evergreen pages.

- [ ] **Step 2: Verify the build still compiles**

The component is created but not yet used. Still run:
```bash
cd frontend && npm run build
```

Expected: build succeeds. TypeScript will tree-shake the unused component but still type-check it.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/components/PageMeta.tsx
git commit -m "feat(frontend): add PageMeta component for dated pages"
```

---

## Task 5: Integrate `PageMeta` into `ContentPage` and add metadata bar CSS

**Files:**
- Modify: `frontend/src/app/[[...slug]]/ContentPage.tsx` (two changes: new import, modified success-branch JSX)
- Modify: `frontend/src/app/globals.css` (append metadata bar styles)

- [ ] **Step 1: Update `ContentPage.tsx` imports**

In `frontend/src/app/[[...slug]]/ContentPage.tsx`, add the `PageMeta` import alongside the existing imports at the top of the file:

```tsx
import { PageMeta } from "@/components/PageMeta";
```

Place it just after the existing `MarkdownRenderer` import (line 5) so imports stay grouped.

- [ ] **Step 2: Update the success-branch JSX in `ContentPage.tsx`**

Find the final `return (...)` block (currently around lines 86–91, the success branch):

```tsx
  return (
    <>
      <MarkdownRenderer content={page.content} />
      <TerminalPrompt path={page.path} />
    </>
  );
```

Replace it with:

```tsx
  return (
    <>
      <PageMeta page={page} />
      <div className={page.show_date ? "has-meta" : ""}>
        <MarkdownRenderer content={page.content} />
      </div>
      <TerminalPrompt path={page.path} />
    </>
  );
```

The wrapping `<div>` carries `has-meta` only when the metadata bar is actually rendered — the CSS in the next step uses this to tighten `#content`'s top padding so the layout stays consistent with today for evergreen pages.

- [ ] **Step 3: Append metadata bar styles to `globals.css`**

Append to the end of `frontend/src/app/globals.css` (after the heading-anchor block added in Task 3):

```css
/* ——— Page Metadata Bar ——— */
.page-meta {
  max-width: 900px;
  padding: 0 20px;
  margin-top: 70px;
  font-size: 0.8rem;
  color: rgba(255, 255, 255, 0.4);
  letter-spacing: 0.02em;
}

.page-meta .sep {
  margin: 0 8px;
  color: rgba(255, 255, 255, 0.2);
}

/* When the metadata bar is present, tighten the content's top padding
   so the overall top margin stays consistent with the no-bar layout. */
.has-meta #content {
  padding-top: 24px;
}
```

The existing `#content` rule has `padding: 70px 20px 0 20px;` (line 58 of `globals.css`). The `.has-meta #content` rule overrides only `padding-top` to `24px` when the bar is present. Evergreen pages (no `.has-meta` wrapper) keep the original `70px` top padding untouched.

- [ ] **Step 4: Verify the build compiles**

Run:
```bash
cd frontend && npm run build
```

Expected: build succeeds, no TypeScript errors.

- [ ] **Step 5: Manual browser verification**

Start dev server if not running:
```bash
cd frontend && npm run dev
```

Pick or create pages to cover each of these cases. For each, note the condition and the expected visual result.

- **Dated page** (a page saved with `show_date: true`): the metadata bar appears above the content. It shows the correctly-formatted date (e.g., `Mar 12, 2026`), reading time (e.g., `4 min read`), and views (e.g., `127 views`). The first markdown `<h1>` renders just below the bar with its usual top border.

- **Evergreen page** (`show_date: false` — the default for new pages per migration 004): no metadata bar appears. The page looks identical to how it looks before this change; spot-check against git's previous version mentally if unsure.

- **Short page** with `show_date: true` (e.g., a page with just "hello world"): the bar shows `1 min read` — reading time is clamped to a minimum of 1 minute, intentional per spec.

- **Long page** with `show_date: true` (if you have one — e.g., paste a long article): reading time roughly scales at 200 wpm. A 1000-word post should show `5 min read`; a 2000-word post should show `10 min read`.

- **Page with exactly 1 view**: the label reads `1 view`, not `1 views`.

- **Admin editor preview** (`/admin/editor`, logged in): the anchor links still work on headings, but the **metadata bar is not rendered** in the preview panel. The editor preview only uses `MarkdownRenderer`, not `ContentPage`, so `PageMeta` is correctly absent. If you see a bar there, something is wrong — the bar must only appear on public content pages.

- **Narrow viewport** (<940px — resize browser or use devtools): the metadata bar remains on a single line for typical content (short date + short reading time + short view count). It does not visually break the surrounding layout.

- **URL fragment navigation** (sanity re-check from Task 3): visiting `/some-dated-page#heading` on a dated page still scrolls to the heading. The metadata bar does not interfere.

- [ ] **Step 6: Commit**

```bash
git add frontend/src/app/\[\[...slug\]\]/ContentPage.tsx frontend/src/app/globals.css
git commit -m "feat(frontend): show metadata bar on dated pages"
```

---

## Done Criteria

All five tasks committed. Manual verification in Task 3 (anchor links) and Task 5 (metadata bar) all pass. `cd frontend && npm run build` succeeds. `go test ./...` still passes (backend untouched, but worth confirming as a sanity check).
