# Reading Time & Heading Anchors — Design

**Date:** 2026-04-23
**Status:** Approved, ready for implementation plan
**Scope:** Frontend-only feature addition to `frontend/`

## Goal

Add two long-form reading affordances to public content pages:

1. A **metadata bar** above dated posts showing publication date, reading time, and view count.
2. **Hover-reveal anchor links** on every rendered heading, backed by stable heading IDs so that URL fragments (`/my-post#intro`) work.

A table of contents was considered and deliberately excluded from scope — a later spec may revisit it.

## Context

- Frontend renders markdown client-side with `react-markdown` + `remark-gfm` + `rehype-raw` + `rehype-highlight`, all inside `frontend/src/components/MarkdownRenderer.tsx`.
- `PageResponse` (`internal/pages/model.go`) already exposes `view_count`, `created_at`, `updated_at`, `show_date`, and `published` — no new fields or migrations required.
- `show_date` already distinguishes dated blog-style pages from evergreen pages; the metadata bar reuses this flag as its visibility gate.
- The right edge of wide viewports (>1375px) is occupied by an existing minimap. Nothing in this spec touches it.

## Out of Scope

- Table of contents (any form).
- Admin editor preview showing the metadata bar — editor continues to render via the same `MarkdownRenderer` but does not wrap it in `PageMeta`.
- Backend changes. No new API fields, DB columns, or migrations.
- Automated frontend tests. The repo has no frontend test harness; setting one up is a separate concern.
- Length-conditional hiding of reading time (e.g., hiding on <1 min pages). Reading time is always shown when the bar is shown.

## Architecture

Three touch points, each with a single purpose:

| Component | Responsibility | Change |
|-----------|----------------|--------|
| `MarkdownRenderer` | Render markdown → HTML with plugin pipeline | Pipeline gains `rehype-slug` + `rehype-autolink-headings`; public `Props` unchanged |
| `PageMeta` (new) | Render the metadata bar for dated posts | Accepts `PageData`, returns a small row or `null` |
| `ContentPage` | Orchestrate the page view | Always renders `<PageMeta>` above `<MarkdownRenderer>` (the component itself returns `null` when `show_date` is false); wraps content in a `has-meta`-classed div when `show_date` is true |

No other files need structural changes. Styling additions live in `frontend/src/app/globals.css`.

## Rendering Pipeline Changes

In `MarkdownRenderer.tsx`, update the rehype plugin chain from:

```
rehypePlugins: [rehypeRaw, rehypeHighlight]
```

to:

```
rehypePlugins: [rehypeRaw, rehypeSlug, rehypeAutolinkHeadings, rehypeHighlight]
```

Order rationale: `rehypeRaw` materializes any inline HTML first. `rehypeSlug` then assigns `id` attributes to headings (e.g., `## My Section` → `id="my-section"`). `rehypeAutolinkHeadings` uses those IDs to insert anchors. `rehypeHighlight` runs last and operates on `<code>` only, so it is orthogonal to heading processing.

**`rehypeAutolinkHeadings` configuration:**

```ts
{
  behavior: "prepend",
  properties: {
    className: ["heading-anchor"],
    "aria-hidden": "true",
    tabIndex: -1,
  },
  content: { type: "text", value: "#" },
}
```

- `prepend` places the `#` inside the heading, before the heading text.
- `aria-hidden` and `tabIndex: -1` keep screen readers and keyboard-tab order focused on the heading content rather than the decorative anchor.
- `content` is a plain text `#`; visual styling is handled in CSS (see Styling).

Duplicate heading text is handled by `rehype-slug`'s default suffix rule (`-1`, `-2`, …). No extra configuration needed.

**Applies everywhere `MarkdownRenderer` is used**, which includes the admin editor preview. This is intentional: anchor links are a rendering affordance, not a metadata affordance, and they cost nothing to show in preview.

**New dependencies:** `rehype-slug`, `rehype-autolink-headings`. Both are in the same unified/rehype ecosystem as existing plugins, maintained, and zero-config at default.

## `PageMeta` Component

**File:** `frontend/src/components/PageMeta.tsx`.

**Signature:**

```tsx
interface Props { page: PageData }
export function PageMeta({ page }: Props) { ... }
```

**Behavior:**

- If `!page.show_date`, return `null`.
- Otherwise render a single `<div className="page-meta">` containing three spans separated by visually-muted ` · ` separators:
  - **Date:** `page.created_at` formatted via `toLocaleDateString("en-US", { year: "numeric", month: "short", day: "numeric" })` → `Mar 12, 2026`.
  - **Reading time:** computed inline in the component body (no helper function, no `useMemo`, no npm dep):
    ```ts
    const words = page.content.trim().split(/\s+/).filter(Boolean).length;
    const minutes = Math.max(1, Math.ceil(words / 200));
    ```
    Rendered as `${minutes} min read`. Minimum 1 minute — short pages still show "1 min read" by design.
  - **Views:** `${page.view_count} views`, with singular handling: `1 view`.

**Trade-offs accepted for reading time:**
- Counts markdown syntax tokens (`**`, `#`, `[`) as words. At "X min read" granularity this is invisible noise.
- Counts code-block content as prose, which slightly under-reads time for tech posts. Accepted — 200 wpm is already a rough average.

## Rendering Integration

In `frontend/src/app/[[...slug]]/ContentPage.tsx`, inside the success branch that currently renders `<MarkdownRenderer content={page.content} />`, wrap the content in an element that carries a CSS class when the metadata bar is present, then render `<PageMeta>` above it:

```tsx
<>
  <PageMeta page={page} />
  <div className={page.show_date ? "has-meta" : ""}>
    <MarkdownRenderer content={page.content} />
  </div>
  <TerminalPrompt path={page.path} />
</>
```

The `has-meta` class lets CSS tighten `#content`'s top padding only when the bar is present, so evergreen pages continue to look identical to today. See Styling.

`<PageMeta>` renders visually above the markdown's first `<h1>`. This is a deliberate placement choice (see Architecture section of brainstorm): the bar reads as a "page header strip" and avoids mutating the markdown pipeline to inject DOM after the h1.

## Styling

All additions go to `frontend/src/app/globals.css`:

```css
/* Page metadata bar */
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
   so overall top margin stays consistent with the no-bar layout. */
.has-meta #content {
  padding-top: 24px;
}

/* Heading anchor links */
#content .heading-anchor {
  color: rgba(255, 255, 255, 0.25);
  text-decoration: none;
  margin-right: 8px;
  opacity: 0;
  transition: opacity 0.15s;
  /* Reset site-wide #content a gradient-underline rule */
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

**Why the `background: none; padding: 0` reset:** the site-wide `#content a` rule applies a teal gradient-underline and padding meant for prose links. Without the reset, the `#` anchor would inherit that styling and visually clash with the heading.

**Responsive check:** the metadata bar inherits `max-width: 900px` to match `#content`, and degrades gracefully on narrow viewports (already handled by the existing `@media (max-width: 940px)` rule cascade for the surrounding layout).

## Dependencies

Add to `frontend/package.json`:

- `rehype-slug`
- `rehype-autolink-headings`

Both are maintained by the unified/rehype organization and used at zero-config defaults (plus the autolink config shown above).

## Verification Plan

No automated tests (frontend has no harness). Manual verification via `npm run dev` and browser:

- **Dated page** (`show_date: true`): bar appears with correctly-formatted date, reading time, and view count. "1 view" vs "N views" pluralization correct.
- **Evergreen page** (`show_date: false`): bar absent. Layout pixel-identical to current (the `has-meta` class guards the padding change).
- **Short page** (<200 words) with `show_date: true`: shows "1 min read".
- **Long page**: reading time scales at roughly 200 wpm; spot-check plausibility.
- **Headings h1–h6**: each shows a hover-reveal `#` before the heading text. Clicking updates the URL to `/path#slug` and scrolls to the heading.
- **URL refresh with fragment**: navigating to `/path#slug` fresh scrolls to the heading (once the client-side fetch completes and the DOM is in place).
- **Duplicate heading text**: produces `slug`, `slug-1`, `slug-2` IDs automatically.
- **Admin editor preview**: anchor links visible on heading hover, no metadata bar.
- **Narrow viewport** (<940px): metadata bar remains readable and does not overflow.

Backend unchanged — `go test ./...` must still pass. `npm run build` must succeed (TypeScript + Next.js static export).

## Risks & Non-Concerns

- **Fragment scroll timing on first load:** `ContentPage` fetches content client-side; if the browser processes `#slug` before the DOM is populated, the scroll may miss. We intentionally do not preempt this with a `useEffect` re-scroll — address only if observed in practice.
- **Reading time accuracy:** accepted as "close enough" (see trade-offs under `PageMeta`).
- **Plugin bundle size:** `rehype-slug` and `rehype-autolink-headings` are small (a few KB each gzipped). Acceptable for a feature of this visibility.
