# Uploads Manager — Design

**Date:** 2026-04-30
**Status:** Approved (auto-chain: design → plan → execute)
**Scope:** A new admin screen for viewing, uploading, and deleting uploaded images, with live reference detection so unused images can be found and removed safely.

## Goal

Add `/admin/uploads`, a standalone admin page that lets the single admin user:

1. See every file in `uploads/` as a thumbnail grid.
2. See, for each image, how many pages reference it and which ones.
3. Upload new images (drag-and-drop or click-to-select).
4. Delete images, with a confirm dialog when the image is still referenced.

The motivating use case is finding and deleting orphaned images that no page links to.

## Out of Scope

- **Rename.** Filenames are already unique (timestamp prefix on upload), so renaming has no user value here.
- **Bulk delete / multi-select.** A single-item delete flow is enough at current scale.
- **Image editing** (crop, resize, replace contents).
- **Folders / tagging** inside `uploads/`.
- **CDN, lazy loading, image optimization** — orthogonal concerns.
- **Recording references in the database.** A live filesystem scan is sufficient at the project's scale and avoids drift.

## Context

- Existing image handling lives in `internal/images/{service,handler}.go`. It already supports `Upload`, `List`, `Delete` and is wired to `GET|POST /api/images` and `DELETE /api/images/{filename}`, all under `protected` JWT middleware. Public reads go through `GET /uploads/{filename}` (file server on `cfg.UploadsDir`).
- Markdown content lives in `cfg.ContentDir` (default `content/`) as `.md` files. Pages reference uploads via either `![alt](/uploads/<filename>)` markdown links or raw `<img src="/uploads/<filename>">` HTML.
- Filenames produced by `Service.Upload` are of the form `<unix-ts>_<sanitized-original-name>.<ext>`, making them unique and giving an upload time directly from the filename.
- Admin frontend lives in `frontend/src/app/admin/{editor,settings,login}/`. There is no admin shell — each route is its own page; the editor links to `/admin/settings` and `/` from a small header in its left rail.
- The editor's existing inline Upload button (in `editor/page.tsx`) calls `POST /api/images` and inserts a markdown image snippet at the cursor. This is preserved unchanged.

## Architecture

Two coordinated changes:

| Component | Change |
|-----------|--------|
| `internal/images/service.go` | Add `Refs(contentDir) (map[string][]string, error)`, extend `ImageInfo` with `RefCount` and `UploadedAt`, add `ErrInUse`, add `force` parameter to `Delete` |
| `internal/images/handler.go` | Enrich `List` with refs in a single scan, add `Refs(filename)` handler returning the page list, return 409 with the page list on `Delete` when refs exist and `force` is not set |
| `cmd/server/main.go` | Register `GET /api/images/{filename}/refs`; existing `DELETE /api/images/{filename}` accepts `?force=1` |
| `internal/images/service_test.go`, `handler_test.go` | New test cases for refs scan, force-delete, 409 body |
| `frontend/src/app/admin/uploads/page.tsx` | New admin route — grid + details rail + dropzone |
| `frontend/src/app/admin/editor/page.tsx` | Add "uploads" link in the pages-rail header (next to "settings"); read `?path=<page>` query on mount and load that page automatically |
| `frontend/src/lib/types.ts` | Extend `ImageInfo` with `ref_count`, `uploaded_at`; add `ImageRefs` |
| `frontend/src/__tests__/admin-uploads-page.test.tsx` | New tests |

## Backend — Reference Scan

A new method on `Service`:

```go
// Refs walks contentDir once, returning a map from upload filename to the
// list of page paths whose .md content contains that filename as a substring.
// All .md files are scanned, including drafts/unpublished pages.
func (s *Service) Refs(contentDir string) (map[string][]string, error)
```

Implementation:

1. List entries in `s.uploadsDir` (skipping `.gitkeep`) to get the set of filenames to look for.
2. Walk `contentDir` recursively for `.md` files using `filepath.WalkDir`.
3. For each markdown file, read its bytes and for each upload filename run `bytes.Contains`. Page path is the file's relative path under `contentDir` with the `.md` suffix stripped.
4. Return the map, plus the first I/O error encountered (best-effort: if any file fails to read, skip it but record an error to surface to the handler).

A substring match against the bare filename catches both forms (`/uploads/...png` and `<img src="/uploads/...png">`) without any markdown-aware parsing. Filenames are unique enough (Unix timestamp prefix, sanitised original) that false positives are not a realistic concern.

`ImageInfo` gains:

```go
type ImageInfo struct {
    Filename   string    `json:"filename"`
    URL        string    `json:"url"`
    Size       int64     `json:"size"`
    RefCount   int       `json:"ref_count"`
    UploadedAt time.Time `json:"uploaded_at"`
}
```

`UploadedAt` is parsed from the filename's `<unix-ts>_` prefix when present, falling back to the file's `mtime` for any pre-existing files without that prefix.

`Service.List` is extended to take an optional refs map (or to compute one via `Refs`) and populate `RefCount` on each item. The handler decides whether to attach refs or not.

## Backend — Handlers and Routes

| Method | Path | Behavior |
|--------|------|----------|
| `GET` | `/api/images` | One refs scan; returns `ImageInfo[]` with `ref_count` populated. If the refs scan returns a partial error, items still render but with `ref_count: -1` to signal "unknown" to the client |
| `GET` | `/api/images/{filename}/refs` | Returns `{ "filename": "...", "pages": ["blog/post-1", "about"] }`. 404 if the image does not exist |
| `POST` | `/api/images` | Unchanged — single-file upload, response unchanged plus the new `RefCount: 0` and `UploadedAt` fields |
| `DELETE` | `/api/images/{filename}` | If the image is referenced and `force=1` is not in the query, returns 409 with body `{ "error": "in use", "pages": [...] }`. Otherwise 204 |

`ErrInUse`:

```go
var ErrInUse = errors.New("image in use")

// Returned together with the list of page paths.
type InUseError struct {
    Pages []string
}
```

`Service.Delete(filename string, force bool) error` returns `*InUseError` (which wraps `ErrInUse`) when refs exist and `force` is false; the handler detects it via `errors.As`.

All routes stay under the `protected` middleware that the existing image routes already use.

## Frontend — `/admin/uploads`

New route at `frontend/src/app/admin/uploads/page.tsx`. Layout:

```
┌──────────────┬─────────────────────────────────┬──────────────┐
│  left rail   │            grid                 │  detail rail │
│              │                                 │              │
│  view:       │  [thumb][thumb][thumb][thumb]   │  preview     │
│  ★ all       │  [thumb][thumb][thumb][thumb]   │  metadata    │
│  unused (3)  │                                 │  refs list   │
│              │                                 │              │
│  sort:       │                                 │  Copy URL    │
│  ↓ newest    │                                 │  Delete      │
│  name        │                                 │              │
│  size        │                                 │              │
│              │                                 │              │
│  ⬆ dropzone  │                                 │              │
└──────────────┴─────────────────────────────────┴──────────────┘
```

### Left rail

- "uploads" header with `site` / `editor` / `logout` links — same pattern as the editor.
- View filter:
  - **all images** — shows everything.
  - **unused only** — shows entries with `ref_count == 0`. The count next to the label is shown in red so unused images stand out.
- Sort:
  - **newest** (default) — by `uploaded_at` desc.
  - **name** — alphabetical asc.
  - **size** — bytes desc.
- Drop zone at the bottom — accepts dragged files and click-to-open file picker.

### Grid

- 4–6 columns at typical widths, square thumbnails (`object-fit: cover`).
- Reference-count badge in the top-right corner of each thumb. Cyan for `>0`, red for `0`. Tooltip shows "Used in N page(s)" or "Not referenced anywhere".
- Filename below thumbnail, single-line ellipsis. The selected thumb has a cyan outline.
- Header above the grid shows totals: e.g. `42 images · 18.4 MB total · 3 unused (1.2 MB)`.

### Detail rail

- Larger preview of the selected image.
- Metadata: filename (selectable), size, uploaded date.
- "Used in (N)" list — clickable page paths. Clicking navigates to `/admin/editor?path=<page>`; the editor reads the query param on mount and calls its existing `loadPage` (extension to existing logic).
- Buttons: **Copy URL** (writes the public `/uploads/<filename>` URL to the clipboard, fires a small toast), **Delete**.

### Delete flow

- If `ref_count === 0`: click Delete → `DELETE /api/images/{filename}`. On success, remove from grid; show a toast.
- If `ref_count > 0`: click Delete → confirm modal listing every page from `pages` (clickable to open in editor); buttons are **Cancel** and **Delete anyway**. The latter calls `DELETE /api/images/{filename}?force=1`.
- If the API returns 409 unexpectedly (e.g., a stale grid where the user thought the image was unused), use the same modal flow with the freshly-returned `pages` list.
- 404 on delete → toast "image already gone" and silently remove from the grid.

### Upload flow

- Drag a file (or files) onto the dropzone, or click it to open the picker. For each file, call the existing `POST /api/images`. Newly-uploaded images are prepended to the grid as they return; the totals header updates accordingly.
- Errors (413 too large, 400 invalid type) surface as toast messages, the same component the editor uses.

### Editor integration

- Editor's left-rail header gains an `uploads` link next to `settings`.
- Editor reads `?path=<page>` on mount: if present and the path resolves to a real page, it calls `loadPage(path)` instead of waiting for a tree click. This is the only behavior change in `editor/page.tsx`.

### Types

```ts
export interface ImageInfo {
  filename: string;
  url: string;
  size: number;
  ref_count: number;     // -1 means refs scan failed; treat as unknown
  uploaded_at: string;   // RFC3339
}

export interface ImageRefs {
  filename: string;
  pages: string[];       // page paths (no .md, no leading slash)
}
```

## Error Handling

- **Refs scan partial failure**: `GET /api/images` still returns 200; affected items have `ref_count: -1`. The grid renders these with a `?` badge and a tooltip "Couldn't compute references for this image"; Delete on these falls back to the always-show-confirm path.
- **Delete while referenced** without `force`: 409 with the page list — drives the confirm modal.
- **Delete a non-existent image** (race): 404 — the grid removes the entry and shows a toast.
- **Upload validation**: existing 413 / 400 unchanged.
- **Network failure** on any operation: caught at the fetch boundary and surfaced via the toast component used in the editor.
- **Auth expiry**: handled by the existing `apiFetch` helper (refresh-on-401), no special handling here.

## Testing

**Go (`internal/images/`):**

- `Refs` finds markdown links, raw HTML `<img>` tags, references in nested folders, and references in unpublished pages (drafts).
- `Refs` returns the right map even when an image is referenced from multiple pages, and returns an entry with an empty slice for an unused image.
- `Refs` skips non-`.md` files and the `.gitkeep` placeholder.
- `Delete` with refs returns `*InUseError` when `force=false`, succeeds when `force=true`.
- Handler `Delete` returns 409 with `{ "pages": [...] }` body when refs exist and `force` is not set; 204 with `?force=1`.
- Handler `List` includes `ref_count` and `uploaded_at` in JSON.
- Handler `Refs` returns 404 for unknown filenames, 200 with the pages array otherwise.

**Frontend (`frontend/src/__tests__/admin-uploads-page.test.tsx`):**

- Grid renders one entry per `ImageInfo`; ref-count badge shows correct color/text.
- Filter toggle hides referenced items in "unused only" mode.
- Sort changes order in the rendered grid.
- Clicking a thumb populates the detail rail.
- Delete on an unused item issues `DELETE` and removes the row.
- Delete on a referenced item opens the confirm modal with the page list; "Delete anyway" issues `?force=1`.
- A 409 response (race) opens the confirm modal even when the local state showed the item as unused.
- Upload via the dropzone calls `POST /api/images` and prepends the response to the grid.

## File Touch List

| File | New / changed |
|------|---------------|
| `internal/images/service.go` | Changed |
| `internal/images/handler.go` | Changed |
| `internal/images/service_test.go` | Changed |
| `internal/images/handler_test.go` | Changed |
| `cmd/server/main.go` | Changed (one new route registration) |
| `frontend/src/app/admin/uploads/page.tsx` | New |
| `frontend/src/app/admin/editor/page.tsx` | Changed (header link + `?path=` on mount) |
| `frontend/src/lib/types.ts` | Changed |
| `frontend/src/__tests__/admin-uploads-page.test.tsx` | New |

## Open Decisions Resolved

- **Reference detection**: live filesystem scan, not an indexed table. Tiny dataset; no drift risk.
- **Drafts count as references**: yes. Substring match across all `.md` files.
- **Rename**: dropped — filenames already unique.
- **Deletion safety**: warn-and-confirm. Block by default with 409, override with `?force=1`.
- **UI placement**: standalone `/admin/uploads` page, not a drawer in the editor. Cleanup is a different mental task from writing.
