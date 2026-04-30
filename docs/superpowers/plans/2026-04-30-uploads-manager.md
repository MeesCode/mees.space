# Uploads Manager Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a `/admin/uploads` admin screen for viewing, uploading, and deleting uploaded images with live reference detection across the markdown content tree.

**Architecture:** Backend gets a `Refs(contentDir)` filesystem-scan method on `images.Service`, an `InUseError` returned from `Delete` when refs exist, and two route changes (enriched `GET /api/images`, new `GET /api/images/{filename}/refs`, `?force=1` on `DELETE`). Frontend gets a new admin route at `frontend/src/app/admin/uploads/page.tsx` plus a small editor change so the rail header links to uploads and the editor honours `?path=` for cross-page navigation.

**Tech Stack:** Go 1.26 standard library, modernc.org/sqlite (untouched here), Next.js 15 + React 19 + TypeScript, Vitest + jsdom + @testing-library/react.

**Spec:** `docs/superpowers/specs/2026-04-30-uploads-manager-design.md`

---

## File Structure

**Create:**
- `frontend/src/app/admin/uploads/page.tsx` — the new admin screen (grid + filters + dropzone + detail rail + delete confirm).
- `frontend/src/app/admin/uploads/page.test.tsx` — Vitest + Testing Library coverage of the screen's behaviours.

**Modify:**
- `internal/images/service.go` — extend `ImageInfo`, change `List` signature, add `Refs`, add `InUseError`, add `force` param on `Delete`, add `uploadedAtFromName` helper.
- `internal/images/service_test.go` — **new file**; co-locates with existing handler tests.
- `internal/images/handler.go` — call `Refs` from `List`, add `GetRefs`, route `force` query in `Delete`.
- `internal/images/handler_test.go` — extend with `Refs` and force-delete cases.
- `cmd/server/main.go` — register `GET /api/images/{filename}/refs`.
- `frontend/src/lib/types.ts` — extend `ImageInfo`, add `ImageRefs`.
- `frontend/src/app/admin/editor/page.tsx` — add `uploads` link in the pages-rail header next to `settings`; on mount, if `?path=<page>` is set call `loadPage(path)`.

All work happens at the repo root unless noted. Run Go commands from the root, frontend commands from `frontend/`.

**Note on test convention:** Existing frontend tests live next to their subject (`api.ts` ↔ `api.test.ts`). The plan follows that — `page.tsx` ↔ `page.test.tsx` — even though the spec's draft mentioned a `__tests__/` folder.

---

## Task 1: Extend `ImageInfo` with `RefCount` and `UploadedAt`

**Files:**
- Modify: `internal/images/service.go`
- Create: `internal/images/service_test.go`

Adds the two new fields and the helper that derives `UploadedAt` from filenames produced by `Service.Upload` (`<unix-ts>_<...>.<ext>`). `RefCount` defaults to 0; the `Refs` plumbing arrives in Task 2.

- [ ] **Step 1: Write the failing test for the filename-timestamp helper**

Create `internal/images/service_test.go`:

```go
package images

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestUploadedAtFromName_ParsesUnixPrefix(t *testing.T) {
	got := uploadedAtFromName("1700000000_my-image.png", time.Time{})
	want := time.Unix(1700000000, 0).UTC()
	if !got.Equal(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestUploadedAtFromName_FallsBackToMtime(t *testing.T) {
	mtime := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	got := uploadedAtFromName("legacy.png", mtime)
	if !got.Equal(mtime) {
		t.Fatalf("got %v, want %v", got, mtime)
	}
}

func TestUploadedAtFromName_FallsBackOnBadPrefix(t *testing.T) {
	mtime := time.Date(2021, 6, 7, 8, 9, 10, 0, time.UTC)
	got := uploadedAtFromName("notanumber_x.png", mtime)
	if !got.Equal(mtime) {
		t.Fatalf("got %v, want %v", got, mtime)
	}
}
```

- [ ] **Step 2: Run the test to confirm it fails**

```bash
go test ./internal/images -run TestUploadedAtFromName -v
```

Expected: FAIL with `undefined: uploadedAtFromName`.

- [ ] **Step 3: Add the helper, the new fields, and update `List`**

Edit `internal/images/service.go`. The file already imports `time`; add `strconv` to the import block. Replace the `ImageInfo` struct so it reads:

```go
type ImageInfo struct {
	Filename   string    `json:"filename"`
	URL        string    `json:"url"`
	Size       int64     `json:"size"`
	RefCount   int       `json:"ref_count"`
	UploadedAt time.Time `json:"uploaded_at"`
}
```

Add at the bottom of the file:

```go
// uploadedAtFromName parses the leading "<unix-ts>_" produced by Service.Upload.
// Falls back to the supplied modTime for filenames without that prefix or with
// a non-numeric prefix.
func uploadedAtFromName(name string, modTime time.Time) time.Time {
	idx := -1
	for i, r := range name {
		if r == '_' {
			idx = i
			break
		}
	}
	if idx <= 0 {
		return modTime
	}
	secs, err := strconv.ParseInt(name[:idx], 10, 64)
	if err != nil {
		return modTime
	}
	return time.Unix(secs, 0).UTC()
}
```

Change `Service.List` so each `ImageInfo` it builds carries `RefCount: 0` and `UploadedAt: uploadedAtFromName(e.Name(), info.ModTime())`. Also extend the signature to accept a `refs map[string][]string` argument that, when non-nil, populates `RefCount` from `len(refs[filename])`. The body becomes:

```go
func (s *Service) List(refs map[string][]string) ([]ImageInfo, error) {
	entries, err := os.ReadDir(s.uploadsDir)
	if err != nil {
		return nil, fmt.Errorf("read uploads dir: %w", err)
	}

	var images []ImageInfo
	for _, e := range entries {
		if e.IsDir() || e.Name() == ".gitkeep" {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		count := 0
		if refs != nil {
			count = len(refs[e.Name()])
		}
		images = append(images, ImageInfo{
			Filename:   e.Name(),
			URL:        "/uploads/" + e.Name(),
			Size:       info.Size(),
			RefCount:   count,
			UploadedAt: uploadedAtFromName(e.Name(), info.ModTime()),
		})
	}

	return images, nil
}
```

`Service.Upload` already returns an `*ImageInfo`; update its return literal to set `RefCount: 0` and `UploadedAt: uploadedAtFromName(filename, info.ModTime())`.

- [ ] **Step 4: Run the helper tests to verify they pass**

```bash
go test ./internal/images -run TestUploadedAtFromName -v
```

Expected: PASS (3 tests).

- [ ] **Step 5: Update existing handler tests for the new `List` signature**

Edit `internal/images/handler.go`. Update the `List` handler so it calls `h.svc.List(nil)` (refs map plumbing comes in Task 4):

```go
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	images, err := h.svc.List(nil)
	if err != nil {
		httputil.JSONError(w, "failed to list images", http.StatusInternalServerError)
		return
	}

	if images == nil {
		images = []ImageInfo{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(images)
}
```

- [ ] **Step 6: Add a service-level test that the new fields are populated**

Append to `internal/images/service_test.go`:

```go
func TestServiceList_PopulatesNewFields(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "1700000000_x.png"), []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}

	svc := NewService(dir)
	got, err := svc.List(nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1 image, got %d", len(got))
	}
	if got[0].RefCount != 0 {
		t.Errorf("default RefCount: want 0, got %d", got[0].RefCount)
	}
	if got[0].UploadedAt.Unix() != 1700000000 {
		t.Errorf("UploadedAt: want unix 1700000000, got %v", got[0].UploadedAt)
	}
}

func TestServiceList_RefsMapPopulatesRefCount(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "1700000000_a.png"), []byte("data"), 0644)
	os.WriteFile(filepath.Join(dir, "1700000001_b.png"), []byte("data"), 0644)

	svc := NewService(dir)
	refs := map[string][]string{
		"1700000000_a.png": {"blog/post-1", "about"},
	}
	got, err := svc.List(refs)
	if err != nil {
		t.Fatal(err)
	}

	counts := map[string]int{}
	for _, im := range got {
		counts[im.Filename] = im.RefCount
	}
	if counts["1700000000_a.png"] != 2 {
		t.Errorf("want 2 refs for a.png, got %d", counts["1700000000_a.png"])
	}
	if counts["1700000001_b.png"] != 0 {
		t.Errorf("want 0 refs for b.png, got %d", counts["1700000001_b.png"])
	}
}
```

- [ ] **Step 7: Run the full images test suite**

```bash
go test ./internal/images -v
```

Expected: all tests PASS, including the existing handler suite (which uses `h.svc.List(nil)` indirectly via the handler).

- [ ] **Step 8: Commit**

```bash
git add internal/images/service.go internal/images/service_test.go internal/images/handler.go
git commit -m "feat(images): extend ImageInfo with RefCount and UploadedAt

ImageInfo gains a ref count (default 0) and an uploaded_at timestamp
parsed from the upload filename's <unix-ts>_ prefix, with mtime fallback.
Service.List now takes a refs map; the handler currently passes nil.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 2: Add `Service.Refs` reference scanner

**Files:**
- Modify: `internal/images/service.go`
- Modify: `internal/images/service_test.go`

A single filesystem walk that returns `map[uploadFilename][]pagePath`. Substring match on the bare filename catches both markdown links and raw `<img>` tags. Drafts are included.

- [ ] **Step 1: Write the failing test for `Refs`**

Append to `internal/images/service_test.go`:

```go
func writeFile(t *testing.T, root, rel, body string) {
	t.Helper()
	full := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestRefs_FindsMarkdownAndRawHTML(t *testing.T) {
	uploads := t.TempDir()
	content := t.TempDir()

	os.WriteFile(filepath.Join(uploads, "1700000000_a.png"), []byte("x"), 0644)
	os.WriteFile(filepath.Join(uploads, "1700000001_b.png"), []byte("x"), 0644)
	os.WriteFile(filepath.Join(uploads, "1700000002_c.png"), []byte("x"), 0644)

	writeFile(t, content, "blog/post.md", "# hi\n\n![alt](/uploads/1700000000_a.png)\n")
	writeFile(t, content, "about.md", `<img src="/uploads/1700000001_b.png">`)
	writeFile(t, content, "drafts/wip.md", "draft using /uploads/1700000000_a.png too")

	svc := NewService(uploads)
	got, err := svc.Refs(content)
	if err != nil {
		t.Fatal(err)
	}

	wantA := map[string]bool{"blog/post": true, "drafts/wip": true}
	for _, p := range got["1700000000_a.png"] {
		delete(wantA, p)
	}
	if len(wantA) != 0 {
		t.Errorf("a.png missing refs: %v (got %v)", wantA, got["1700000000_a.png"])
	}

	if len(got["1700000001_b.png"]) != 1 || got["1700000001_b.png"][0] != "about" {
		t.Errorf("b.png: want [about], got %v", got["1700000001_b.png"])
	}

	if len(got["1700000002_c.png"]) != 0 {
		t.Errorf("c.png: want no refs, got %v", got["1700000002_c.png"])
	}
}

func TestRefs_SkipsNonMarkdownAndGitkeep(t *testing.T) {
	uploads := t.TempDir()
	content := t.TempDir()

	os.WriteFile(filepath.Join(uploads, "1700000000_a.png"), []byte("x"), 0644)
	os.WriteFile(filepath.Join(uploads, ".gitkeep"), nil, 0644)

	writeFile(t, content, "post.md", "/uploads/1700000000_a.png")
	writeFile(t, content, "post.txt", "/uploads/1700000000_a.png")
	writeFile(t, content, ".gitkeep", "")

	svc := NewService(uploads)
	got, err := svc.Refs(content)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := got[".gitkeep"]; ok {
		t.Error(".gitkeep should not be in refs map")
	}
	if len(got["1700000000_a.png"]) != 1 {
		t.Errorf("want 1 ref (only post.md), got %v", got["1700000000_a.png"])
	}
}

func TestRefs_ReturnsEmptySliceForUnusedImage(t *testing.T) {
	uploads := t.TempDir()
	content := t.TempDir()

	os.WriteFile(filepath.Join(uploads, "1700000000_orphan.png"), []byte("x"), 0644)
	writeFile(t, content, "post.md", "no images here")

	svc := NewService(uploads)
	got, err := svc.Refs(content)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := got["1700000000_orphan.png"]; !ok {
		t.Fatal("orphan image must have an entry in the map")
	}
	if len(got["1700000000_orphan.png"]) != 0 {
		t.Errorf("want empty slice, got %v", got["1700000000_orphan.png"])
	}
}
```

- [ ] **Step 2: Run the tests to confirm they fail**

```bash
go test ./internal/images -run TestRefs -v
```

Expected: FAIL — `svc.Refs undefined`.

- [ ] **Step 3: Implement `Refs` on `Service`**

Add to `internal/images/service.go`. Make sure `bytes`, `io/fs`, and `strings` are in the import block.

```go
// Refs walks contentDir for .md files and returns a map from each upload
// filename to the page paths whose contents contain that filename as a
// substring. Drafts and unpublished pages are scanned just like any other
// .md file. The map always has an entry for every non-.gitkeep file in
// the uploads directory; unused images map to an empty slice.
//
// On a per-file read error the file is skipped and the first such error is
// returned alongside the (still useful) partial map. Callers that need a
// fully trustworthy result should treat any non-nil error as "unknown".
func (s *Service) Refs(contentDir string) (map[string][]string, error) {
	entries, err := os.ReadDir(s.uploadsDir)
	if err != nil {
		return nil, fmt.Errorf("read uploads dir: %w", err)
	}

	out := make(map[string][]string, len(entries))
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || e.Name() == ".gitkeep" {
			continue
		}
		out[e.Name()] = nil
		names = append(names, e.Name())
	}

	if len(names) == 0 {
		return out, nil
	}

	var firstErr error
	walkErr := filepath.WalkDir(contentDir, func(path string, d fs.DirEntry, werr error) error {
		if werr != nil {
			if firstErr == nil {
				firstErr = werr
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(d.Name()), ".md") {
			return nil
		}
		body, readErr := os.ReadFile(path)
		if readErr != nil {
			if firstErr == nil {
				firstErr = readErr
			}
			return nil
		}
		rel, relErr := filepath.Rel(contentDir, path)
		if relErr != nil {
			rel = path
		}
		page := strings.TrimSuffix(filepath.ToSlash(rel), ".md")
		for _, name := range names {
			if bytes.Contains(body, []byte(name)) {
				out[name] = append(out[name], page)
			}
		}
		return nil
	})
	if firstErr == nil {
		firstErr = walkErr
	}
	return out, firstErr
}
```

- [ ] **Step 4: Run the new tests to verify they pass**

```bash
go test ./internal/images -run TestRefs -v
```

Expected: PASS.

- [ ] **Step 5: Run the full images suite**

```bash
go test ./internal/images -v
```

Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/images/service.go internal/images/service_test.go
git commit -m "feat(images): add Service.Refs filesystem scanner

Walks contentDir for .md files and substring-matches each upload's
filename against the file bytes, building map[filename][]pagePath.
Catches both markdown image links and raw <img src=...>. Includes
drafts.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 3: Force-delete with `InUseError`

**Files:**
- Modify: `internal/images/service.go`
- Modify: `internal/images/service_test.go`

`Service.Delete` gains a `force bool` parameter. When the image has refs and `force` is false, it returns `*InUseError` (which wraps `ErrInUse`). The handler will use this in Task 6.

- [ ] **Step 1: Write the failing tests**

Add `errors` to the import block of `internal/images/service_test.go` (alongside `os`, `path/filepath`, `testing`, `time`), then append:

```go
func TestDelete_RejectsWhenInUse(t *testing.T) {
	uploads := t.TempDir()
	content := t.TempDir()

	os.WriteFile(filepath.Join(uploads, "1700000000_a.png"), []byte("x"), 0644)
	writeFile(t, content, "blog/post.md", "/uploads/1700000000_a.png")

	svc := NewService(uploads)
	refs, err := svc.Refs(content)
	if err != nil {
		t.Fatal(err)
	}

	delErr := svc.Delete("1700000000_a.png", false, refs["1700000000_a.png"])
	if delErr == nil {
		t.Fatal("want InUseError, got nil")
	}
	var inUse *InUseError
	if !errors.As(delErr, &inUse) {
		t.Fatalf("want *InUseError, got %T (%v)", delErr, delErr)
	}
	if !errors.Is(delErr, ErrInUse) {
		t.Errorf("error must wrap ErrInUse: %v", delErr)
	}
	if len(inUse.Pages) != 1 || inUse.Pages[0] != "blog/post" {
		t.Errorf("want pages [blog/post], got %v", inUse.Pages)
	}
	if _, statErr := os.Stat(filepath.Join(uploads, "1700000000_a.png")); statErr != nil {
		t.Error("file must NOT be deleted on rejection")
	}
}

func TestDelete_ForceRemovesEvenWhenInUse(t *testing.T) {
	uploads := t.TempDir()
	os.WriteFile(filepath.Join(uploads, "1700000000_a.png"), []byte("x"), 0644)

	svc := NewService(uploads)
	if err := svc.Delete("1700000000_a.png", true, []string{"blog/post"}); err != nil {
		t.Fatalf("force delete failed: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(uploads, "1700000000_a.png")); !os.IsNotExist(statErr) {
		t.Error("file should be deleted")
	}
}

func TestDelete_NoRefsAlwaysRemoves(t *testing.T) {
	uploads := t.TempDir()
	os.WriteFile(filepath.Join(uploads, "1700000000_a.png"), []byte("x"), 0644)

	svc := NewService(uploads)
	if err := svc.Delete("1700000000_a.png", false, nil); err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(uploads, "1700000000_a.png")); !os.IsNotExist(statErr) {
		t.Error("file should be deleted")
	}
}
```

- [ ] **Step 2: Run the tests to confirm they fail**

```bash
go test ./internal/images -run TestDelete -v
```

Expected: FAIL — Delete signature doesn't match (3 args expected) and `InUseError`/`ErrInUse` undefined.

- [ ] **Step 3: Update the service**

Edit `internal/images/service.go`. Add to the `var (...)` block:

```go
ErrInUse = errors.New("image in use")
```

Make sure `errors` is imported.

Add the error type below it:

```go
// InUseError reports that a delete was rejected because the image is still
// referenced. It wraps ErrInUse and carries the affected page paths.
type InUseError struct {
	Pages []string
}

func (e *InUseError) Error() string { return ErrInUse.Error() }
func (e *InUseError) Unwrap() error { return ErrInUse }
```

Replace the existing `Delete` method:

```go
// Delete removes filename from the uploads directory. If pages is non-empty
// and force is false, returns *InUseError without touching the file.
func (s *Service) Delete(filename string, force bool, pages []string) error {
	if strings.Contains(filename, "/") || strings.Contains(filename, "\\") || strings.Contains(filename, "..") {
		return ErrNotFound
	}

	path := filepath.Join(s.uploadsDir, filename)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return ErrNotFound
	}

	if !force && len(pages) > 0 {
		return &InUseError{Pages: pages}
	}
	return os.Remove(path)
}
```

- [ ] **Step 4: Update the handler call site so the package builds**

Edit `internal/images/handler.go`. Replace the body of `Delete`:

```go
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	filename := r.PathValue("filename")
	if filename == "" {
		httputil.JSONError(w, "filename required", http.StatusBadRequest)
		return
	}

	err := h.svc.Delete(filename, true, nil)
	if err == ErrNotFound {
		httputil.JSONError(w, "image not found", http.StatusNotFound)
		return
	}
	if err != nil {
		httputil.JSONError(w, "failed to delete image", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
```

(`force=true, pages=nil` matches the previous always-delete behaviour. Task 6 wires the real refs-aware behaviour.)

- [ ] **Step 5: Run the new tests to verify they pass**

```bash
go test ./internal/images -run TestDelete -v
```

Expected: PASS.

- [ ] **Step 6: Run the full suite**

```bash
go test ./internal/images -v
```

Expected: all PASS, including `TestDeleteImage` and `TestDeleteImage_NotFound` from `handler_test.go`.

- [ ] **Step 7: Commit**

```bash
git add internal/images/service.go internal/images/service_test.go internal/images/handler.go
git commit -m "feat(images): force flag and InUseError on Service.Delete

Service.Delete(filename, force, pages) returns *InUseError (wrapping
ErrInUse) when refs are present and force is false. Handler still calls
Delete with force=true; the refs-aware path lands in a follow-up.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 4: Enrich `GET /api/images` with refs

**Files:**
- Modify: `internal/images/handler.go`
- Modify: `internal/images/handler_test.go`

`List` now scans references once per request. If `Refs` errors, every item's `RefCount` is `-1` (unknown).

- [ ] **Step 1: Update handler-test setup so tests can inject a content directory**

Open `internal/images/handler_test.go`. Notice the existing helpers construct `NewService(uploadsDir)` and `NewHandler(svc)`. The handler currently has no notion of a content directory, so we must thread one through.

Edit `internal/images/handler.go`:

```go
type Handler struct {
	svc        *Service
	contentDir string
}

func NewHandler(svc *Service, contentDir string) *Handler {
	return &Handler{svc: svc, contentDir: contentDir}
}
```

Update every `NewHandler(svc)` callsite in `handler_test.go` to `NewHandler(svc, "")` — there are five (in `TestUploadImage`, `TestUploadInvalidType`, `TestListImages`, `TestDeleteImage`, `TestDeleteImage_NotFound`). An empty `contentDir` means `Refs` walks a nonexistent tree and the resulting map has zero refs for every image — fine for the existing tests, which don't care about counts.

Also update the production callsite in `cmd/server/main.go`:

```go
imagesHandler := images.NewHandler(imagesSvc, cfg.ContentDir)
```

- [ ] **Step 2: Write the failing test for ref_count enrichment**

Append to `internal/images/handler_test.go`:

```go
func TestListImages_PopulatesRefCount(t *testing.T) {
	uploads := t.TempDir()
	content := t.TempDir()

	os.WriteFile(filepath.Join(uploads, "1700000000_a.png"), []byte("x"), 0644)
	os.WriteFile(filepath.Join(uploads, "1700000001_b.png"), []byte("x"), 0644)
	os.MkdirAll(filepath.Join(content, "blog"), 0755)
	os.WriteFile(filepath.Join(content, "blog", "post.md"),
		[]byte("![](/uploads/1700000000_a.png)"), 0644)

	svc := NewService(uploads)
	h := NewHandler(svc, content)

	req := httptest.NewRequest("GET", "/api/images", nil)
	rr := httptest.NewRecorder()
	h.List(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rr.Code)
	}

	var images []ImageInfo
	json.NewDecoder(rr.Body).Decode(&images)

	counts := map[string]int{}
	for _, im := range images {
		counts[im.Filename] = im.RefCount
	}
	if counts["1700000000_a.png"] != 1 {
		t.Errorf("a.png ref_count: want 1, got %d", counts["1700000000_a.png"])
	}
	if counts["1700000001_b.png"] != 0 {
		t.Errorf("b.png ref_count: want 0, got %d", counts["1700000001_b.png"])
	}
}
```

- [ ] **Step 3: Run the test to confirm it fails**

```bash
go test ./internal/images -run TestListImages_PopulatesRefCount -v
```

Expected: FAIL — both ref counts are 0 because `List` is still called with `nil`.

- [ ] **Step 4: Wire `Refs` into the `List` handler**

Replace the `List` method body in `internal/images/handler.go`:

```go
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	refs, refsErr := h.svc.Refs(h.contentDir)
	images, err := h.svc.List(refs)
	if err != nil {
		httputil.JSONError(w, "failed to list images", http.StatusInternalServerError)
		return
	}

	if images == nil {
		images = []ImageInfo{}
	}

	if refsErr != nil {
		for i := range images {
			images[i].RefCount = -1
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(images)
}
```

- [ ] **Step 5: Run the test to verify it passes**

```bash
go test ./internal/images -run TestListImages_PopulatesRefCount -v
```

Expected: PASS.

- [ ] **Step 6: Run the full Go test suite**

```bash
go test ./...
```

Expected: all PASS. (`cmd/server/main_test.go` exercises the full mux; the new `NewHandler(svc, cfg.ContentDir)` callsite must compile.)

- [ ] **Step 7: Commit**

```bash
git add internal/images/handler.go internal/images/handler_test.go cmd/server/main.go
git commit -m "feat(images): GET /api/images returns ref_count per image

The list handler now runs Service.Refs once per request and populates
ImageInfo.RefCount. On scan failure every ref_count is set to -1
(unknown). Threaded contentDir through Handler.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 5: `GET /api/images/{filename}/refs` endpoint

**Files:**
- Modify: `internal/images/handler.go`
- Modify: `internal/images/handler_test.go`
- Modify: `cmd/server/main.go`

Per-image refs endpoint used by the delete confirm modal and the detail rail's "Used in" list.

- [ ] **Step 1: Write the failing tests**

Append to `internal/images/handler_test.go`:

```go
func TestGetRefs_OK(t *testing.T) {
	uploads := t.TempDir()
	content := t.TempDir()

	os.WriteFile(filepath.Join(uploads, "1700000000_a.png"), []byte("x"), 0644)
	os.MkdirAll(filepath.Join(content, "blog"), 0755)
	os.WriteFile(filepath.Join(content, "blog", "post.md"),
		[]byte("/uploads/1700000000_a.png"), 0644)
	os.WriteFile(filepath.Join(content, "about.md"),
		[]byte("/uploads/1700000000_a.png"), 0644)

	svc := NewService(uploads)
	h := NewHandler(svc, content)

	req := httptest.NewRequest("GET", "/api/images/1700000000_a.png/refs", nil)
	req.SetPathValue("filename", "1700000000_a.png")
	rr := httptest.NewRecorder()
	h.GetRefs(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var body struct {
		Filename string   `json:"filename"`
		Pages    []string `json:"pages"`
	}
	json.NewDecoder(rr.Body).Decode(&body)
	if body.Filename != "1700000000_a.png" {
		t.Errorf("filename: %q", body.Filename)
	}
	want := map[string]bool{"blog/post": true, "about": true}
	for _, p := range body.Pages {
		delete(want, p)
	}
	if len(want) != 0 {
		t.Errorf("missing pages: %v (got %v)", want, body.Pages)
	}
}

func TestGetRefs_NotFound(t *testing.T) {
	uploads := t.TempDir()
	svc := NewService(uploads)
	h := NewHandler(svc, t.TempDir())

	req := httptest.NewRequest("GET", "/api/images/nope.png/refs", nil)
	req.SetPathValue("filename", "nope.png")
	rr := httptest.NewRecorder()
	h.GetRefs(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d", rr.Code)
	}
}
```

- [ ] **Step 2: Run the tests to confirm they fail**

```bash
go test ./internal/images -run TestGetRefs -v
```

Expected: FAIL — `h.GetRefs undefined`.

- [ ] **Step 3: Implement `GetRefs`**

Append to `internal/images/handler.go`:

```go
func (h *Handler) GetRefs(w http.ResponseWriter, r *http.Request) {
	filename := r.PathValue("filename")
	if filename == "" {
		httputil.JSONError(w, "filename required", http.StatusBadRequest)
		return
	}
	if strings.Contains(filename, "/") || strings.Contains(filename, "\\") || strings.Contains(filename, "..") {
		httputil.JSONError(w, "image not found", http.StatusNotFound)
		return
	}
	if _, err := os.Stat(filepath.Join(h.svc.uploadsDir, filename)); os.IsNotExist(err) {
		httputil.JSONError(w, "image not found", http.StatusNotFound)
		return
	}

	refs, err := h.svc.Refs(h.contentDir)
	if err != nil {
		httputil.JSONError(w, "failed to scan references", http.StatusInternalServerError)
		return
	}

	pages := refs[filename]
	if pages == nil {
		pages = []string{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"filename": filename,
		"pages":    pages,
	})
}
```

Make sure `os`, `path/filepath`, and `strings` are in the import block.

- [ ] **Step 4: Register the route**

Edit `cmd/server/main.go`. In the Images section, add a single line so the block reads:

```go
// Images
mux.Handle("GET /api/images", protected(imagesHandler.List))
mux.Handle("POST /api/images", protected(imagesHandler.Upload))
mux.Handle("GET /api/images/{filename}/refs", protected(imagesHandler.GetRefs))
mux.Handle("DELETE /api/images/{filename}", protected(imagesHandler.Delete))
```

- [ ] **Step 5: Run the new tests to verify they pass**

```bash
go test ./internal/images -run TestGetRefs -v
```

Expected: PASS.

- [ ] **Step 6: Run all backend tests**

```bash
go test ./...
```

Expected: all PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/images/handler.go internal/images/handler_test.go cmd/server/main.go
git commit -m "feat(images): GET /api/images/{filename}/refs endpoint

Returns {filename, pages[]} for one image. 404 when the file does not
exist. Used by the delete-confirm dialog and the detail-rail refs list.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 6: Refs-aware `DELETE /api/images/{filename}`

**Files:**
- Modify: `internal/images/handler.go`
- Modify: `internal/images/handler_test.go`

Without `?force=1`, the handler refuses to delete an image that has refs and returns 409 with the page list. With `?force=1`, deletes regardless.

- [ ] **Step 1: Write the failing tests**

Append to `internal/images/handler_test.go`:

```go
func TestDeleteImage_409WhenInUse(t *testing.T) {
	uploads := t.TempDir()
	content := t.TempDir()

	os.WriteFile(filepath.Join(uploads, "1700000000_a.png"), []byte("x"), 0644)
	os.MkdirAll(filepath.Join(content, "blog"), 0755)
	os.WriteFile(filepath.Join(content, "blog", "post.md"),
		[]byte("/uploads/1700000000_a.png"), 0644)

	svc := NewService(uploads)
	h := NewHandler(svc, content)

	req := httptest.NewRequest("DELETE", "/api/images/1700000000_a.png", nil)
	req.SetPathValue("filename", "1700000000_a.png")
	rr := httptest.NewRecorder()
	h.Delete(rr, req)

	if rr.Code != http.StatusConflict {
		t.Fatalf("want 409, got %d", rr.Code)
	}

	var body struct {
		Error string   `json:"error"`
		Pages []string `json:"pages"`
	}
	json.NewDecoder(rr.Body).Decode(&body)
	if body.Error != "in use" {
		t.Errorf(`error: want "in use", got %q`, body.Error)
	}
	if len(body.Pages) != 1 || body.Pages[0] != "blog/post" {
		t.Errorf("pages: want [blog/post], got %v", body.Pages)
	}

	if _, statErr := os.Stat(filepath.Join(uploads, "1700000000_a.png")); statErr != nil {
		t.Error("file must NOT be deleted on 409")
	}
}

func TestDeleteImage_ForceQueryRemoves(t *testing.T) {
	uploads := t.TempDir()
	content := t.TempDir()

	os.WriteFile(filepath.Join(uploads, "1700000000_a.png"), []byte("x"), 0644)
	os.MkdirAll(filepath.Join(content, "blog"), 0755)
	os.WriteFile(filepath.Join(content, "blog", "post.md"),
		[]byte("/uploads/1700000000_a.png"), 0644)

	svc := NewService(uploads)
	h := NewHandler(svc, content)

	req := httptest.NewRequest("DELETE", "/api/images/1700000000_a.png?force=1", nil)
	req.SetPathValue("filename", "1700000000_a.png")
	rr := httptest.NewRecorder()
	h.Delete(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("want 204, got %d: %s", rr.Code, rr.Body.String())
	}
	if _, statErr := os.Stat(filepath.Join(uploads, "1700000000_a.png")); !os.IsNotExist(statErr) {
		t.Error("file should be deleted")
	}
}
```

- [ ] **Step 2: Run the new tests to confirm they fail**

```bash
go test ./internal/images -run TestDeleteImage_ -v
```

Expected: `409WhenInUse` FAILS (currently returns 204), `ForceQueryRemoves` PASSES already (delete still works), and the existing `TestDeleteImage` and `TestDeleteImage_NotFound` continue to pass.

- [ ] **Step 3: Make the `Delete` handler refs-aware**

Replace the `Delete` body in `internal/images/handler.go`:

```go
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	filename := r.PathValue("filename")
	if filename == "" {
		httputil.JSONError(w, "filename required", http.StatusBadRequest)
		return
	}
	force := r.URL.Query().Get("force") == "1"

	var pages []string
	if !force {
		refs, refsErr := h.svc.Refs(h.contentDir)
		if refsErr != nil {
			httputil.JSONError(w, "failed to scan references", http.StatusInternalServerError)
			return
		}
		pages = refs[filename]
	}

	err := h.svc.Delete(filename, force, pages)
	if err == ErrNotFound {
		httputil.JSONError(w, "image not found", http.StatusNotFound)
		return
	}
	var inUse *InUseError
	if errors.As(err, &inUse) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(map[string]any{
			"error": "in use",
			"pages": inUse.Pages,
		})
		return
	}
	if err != nil {
		httputil.JSONError(w, "failed to delete image", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
```

Add `errors` to the import block if it isn't there.

- [ ] **Step 4: Run all images tests**

```bash
go test ./internal/images -v
```

Expected: all PASS, including the existing `TestDeleteImage` (no refs → 204) and `TestDeleteImage_NotFound` (404).

- [ ] **Step 5: Run the full Go suite**

```bash
go test ./...
```

Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/images/handler.go internal/images/handler_test.go
git commit -m "feat(images): DELETE returns 409 when image is referenced

Without ?force=1 the delete handler runs Service.Refs and rejects with
409 + {error, pages[]} when the image is referenced. ?force=1 deletes
unconditionally.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 7: Frontend types and editor `?path=` integration

**Files:**
- Modify: `frontend/src/lib/types.ts`
- Modify: `frontend/src/app/admin/editor/page.tsx`

Tiny change but it unblocks the frontend test in Task 9.

- [ ] **Step 1: Extend `ImageInfo` and add `ImageRefs`**

Replace the `ImageInfo` block in `frontend/src/lib/types.ts`:

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

- [ ] **Step 2: Add the `uploads` link in the editor's pages-rail header**

Edit `frontend/src/app/admin/editor/page.tsx`. Find the `<a href="/admin/settings">settings</a>` link in the `pages` rail header (around line 447–457 — search for `settings`). Insert a sibling link **before** the settings link:

```tsx
<a
  href="/admin/uploads"
  style={{
    color: "rgba(255,255,255,0.4)",
    textDecoration: "none",
    fontFamily: "inherit",
    fontSize: "0.75rem",
    cursor: "pointer",
  }}
>
  uploads
</a>
```

- [ ] **Step 3: Honour `?path=` on mount**

Still in `editor/page.tsx`, find the existing tree-loading effect:

```tsx
useEffect(() => {
  loadTree();
}, [loadTree]);
```

Add a second effect immediately after it that auto-selects the page given via the query string. Place it before the auto-save effect:

```tsx
useEffect(() => {
  if (typeof window === "undefined") return;
  const sp = new URLSearchParams(window.location.search);
  const path = sp.get("path");
  if (path) {
    loadPage(path);
  }
  // intentionally only on first mount
  // eslint-disable-next-line react-hooks/exhaustive-deps
}, []);
```

- [ ] **Step 4: Type-check the frontend**

```bash
cd frontend && npx tsc --noEmit
```

Expected: no output (clean).

- [ ] **Step 5: Run the existing frontend tests**

```bash
cd frontend && npm test
```

Expected: existing tests pass.

- [ ] **Step 6: Commit**

```bash
git add frontend/src/lib/types.ts frontend/src/app/admin/editor/page.tsx
git commit -m "feat(frontend): add uploads link + ?path= deep-link to editor

types.ts gains ref_count + uploaded_at on ImageInfo and a new ImageRefs
type. The editor's pages rail gets an 'uploads' link next to 'settings',
and on mount it reads ?path= so deep links from the uploads manager
auto-select the page.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 8: `/admin/uploads` page — grid skeleton + tests

**Files:**
- Create: `frontend/src/app/admin/uploads/page.tsx`
- Create: `frontend/src/app/admin/uploads/page.test.tsx`

This task ships a minimal-but-real page: header, grid of thumbnails with ref-count badges, totals header, and a no-op detail rail. Filters, sort, dropzone, delete, and Copy URL come in Tasks 9 and 10. We do this iteratively because the page is the largest single artifact in the plan.

- [ ] **Step 1: Write the failing test**

Create `frontend/src/app/admin/uploads/page.test.tsx`:

```tsx
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import UploadsPage from "./page";

const sample = [
  {
    filename: "1700000000_a.png",
    url: "/uploads/1700000000_a.png",
    size: 1024,
    ref_count: 2,
    uploaded_at: "2023-11-14T22:13:20Z",
  },
  {
    filename: "1700000001_b.png",
    url: "/uploads/1700000001_b.png",
    size: 2048,
    ref_count: 0,
    uploaded_at: "2023-11-14T22:13:21Z",
  },
];

function installFetchMock(initial: unknown) {
  const fetchMock = vi.fn().mockImplementation((path: string) => {
    if (path === "/api/images") {
      return Promise.resolve(new Response(JSON.stringify(initial), { status: 200 }));
    }
    return Promise.resolve(new Response("", { status: 404 }));
  });
  vi.stubGlobal("fetch", fetchMock);
  return fetchMock;
}

describe("UploadsPage — grid skeleton", () => {
  beforeEach(() => {
    localStorage.setItem("access_token", "tok");
  });

  it("renders one thumbnail per ImageInfo with ref_count badge", async () => {
    installFetchMock(sample);
    render(<UploadsPage />);

    await waitFor(() => {
      expect(screen.getByText("1700000000_a.png")).toBeDefined();
      expect(screen.getByText("1700000001_b.png")).toBeDefined();
    });

    expect(screen.getByTestId("ref-badge-1700000000_a.png").textContent).toBe("2");
    expect(screen.getByTestId("ref-badge-1700000001_b.png").textContent).toBe("0");
  });

  it("shows the totals header", async () => {
    installFetchMock(sample);
    render(<UploadsPage />);

    await waitFor(() => {
      expect(screen.getByTestId("totals").textContent).toContain("2 images");
      expect(screen.getByTestId("totals").textContent).toContain("1 unused");
    });
  });

  it("renders ? badge when ref_count is -1", async () => {
    installFetchMock([{ ...sample[0], ref_count: -1 }]);
    render(<UploadsPage />);

    await waitFor(() => {
      expect(screen.getByTestId("ref-badge-1700000000_a.png").textContent).toBe("?");
    });
  });
});
```

- [ ] **Step 2: Run the test to confirm it fails**

```bash
cd frontend && npm test -- page.test
```

Expected: FAIL — `UploadsPage` cannot be imported.

- [ ] **Step 3: Implement the skeleton page**

Create `frontend/src/app/admin/uploads/page.tsx`:

```tsx
"use client";

import { useEffect, useState } from "react";
import { apiFetch } from "@/lib/api";
import { logout } from "@/lib/auth";
import { ImageInfo } from "@/lib/types";

export default function UploadsPage() {
  const [images, setImages] = useState<ImageInfo[]>([]);
  const [selected, setSelected] = useState<string | null>(null);
  const [loaded, setLoaded] = useState(false);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      const res = await apiFetch("/api/images");
      if (!res.ok) {
        setLoaded(true);
        return;
      }
      const data: ImageInfo[] = await res.json();
      if (!cancelled) {
        setImages(data);
        setLoaded(true);
      }
    })();
    return () => { cancelled = true; };
  }, []);

  const totalBytes = images.reduce((n, im) => n + im.size, 0);
  const unused = images.filter((im) => im.ref_count === 0);
  const unusedBytes = unused.reduce((n, im) => n + im.size, 0);

  return (
    <div style={{ display: "flex", height: "100vh", overflow: "hidden", color: "var(--color)" }}>
      {/* Left rail */}
      <div style={leftRailStyle}>
        <div style={headerStyle}>
          <span style={{ color: "var(--accent)", fontWeight: "bold", fontSize: "0.9rem" }}>
            uploads
          </span>
          <span style={{ display: "flex", gap: 10 }}>
            <a href="/" style={navLinkStyle}>site</a>
            <a href="/admin/editor" style={navLinkStyle}>editor</a>
            <button onClick={logout} style={{ ...navLinkStyle, background: "none", border: "none", cursor: "pointer" }}>
              logout
            </button>
          </span>
        </div>
      </div>

      {/* Grid + totals */}
      <div style={{ flex: 1, padding: 14, overflow: "auto" }}>
        <div data-testid="totals" style={totalsStyle}>
          {loaded
            ? `${images.length} images · ${formatBytes(totalBytes)} total · ${unused.length} unused (${formatBytes(unusedBytes)})`
            : "loading…"}
        </div>
        <div style={gridStyle}>
          {images.map((im) => (
            <div
              key={im.filename}
              onClick={() => setSelected(im.filename)}
              style={{
                position: "relative",
                cursor: "pointer",
                outline: selected === im.filename ? "2px solid var(--accent)" : "none",
              }}
            >
              <img
                src={im.url}
                alt={im.filename}
                loading="lazy"
                style={{
                  width: "100%",
                  aspectRatio: "1",
                  objectFit: "cover",
                  background: "#1a1a1a",
                  display: "block",
                }}
              />
              <div
                data-testid={`ref-badge-${im.filename}`}
                title={refTitle(im.ref_count)}
                style={badgeStyle(im.ref_count)}
              >
                {refLabel(im.ref_count)}
              </div>
              <div style={filenameStyle}>{im.filename}</div>
            </div>
          ))}
        </div>
      </div>

      {/* Detail rail (skeleton — populated by later tasks) */}
      <div style={detailRailStyle}>
        {selected ? <span style={{ fontSize: 12 }}>{selected}</span> : null}
      </div>
    </div>
  );
}

function refLabel(count: number): string {
  if (count < 0) return "?";
  return String(count);
}

function refTitle(count: number): string {
  if (count < 0) return "Couldn't compute references";
  if (count === 0) return "Not referenced anywhere";
  return `Used in ${count} page${count === 1 ? "" : "s"}`;
}

function badgeStyle(count: number): React.CSSProperties {
  let bg = "var(--accent)";
  if (count === 0) bg = "#ff6b6b";
  if (count < 0) bg = "rgba(255,255,255,0.3)";
  return {
    position: "absolute",
    top: 4,
    right: 4,
    background: bg,
    color: "#000",
    fontSize: 10,
    padding: "1px 5px",
    borderRadius: 2,
  };
}

function formatBytes(n: number): string {
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
  return `${(n / 1024 / 1024).toFixed(1)} MB`;
}

const leftRailStyle: React.CSSProperties = {
  width: 170,
  borderRight: "1px solid rgba(255,255,255,0.08)",
  padding: 14,
  flexShrink: 0,
  overflow: "auto",
};

const headerStyle: React.CSSProperties = {
  display: "flex",
  justifyContent: "space-between",
  alignItems: "center",
  marginBottom: 16,
};

const navLinkStyle: React.CSSProperties = {
  color: "rgba(255,255,255,0.4)",
  textDecoration: "none",
  fontFamily: "inherit",
  fontSize: "0.75rem",
};

const totalsStyle: React.CSSProperties = {
  fontSize: 11,
  color: "rgba(255,255,255,0.5)",
  marginBottom: 10,
};

const gridStyle: React.CSSProperties = {
  display: "grid",
  gridTemplateColumns: "repeat(auto-fill, minmax(120px, 1fr))",
  gap: 8,
};

const filenameStyle: React.CSSProperties = {
  fontSize: 10,
  color: "rgba(255,255,255,0.45)",
  marginTop: 3,
  whiteSpace: "nowrap",
  overflow: "hidden",
  textOverflow: "ellipsis",
};

const detailRailStyle: React.CSSProperties = {
  width: 230,
  borderLeft: "1px solid rgba(255,255,255,0.08)",
  padding: 14,
  flexShrink: 0,
};
```

- [ ] **Step 4: Run the test to verify it passes**

```bash
cd frontend && npm test -- page.test
```

Expected: all three tests in `UploadsPage — grid skeleton` PASS.

- [ ] **Step 5: Type-check**

```bash
cd frontend && npx tsc --noEmit
```

Expected: clean.

- [ ] **Step 6: Commit**

```bash
git add frontend/src/app/admin/uploads/page.tsx frontend/src/app/admin/uploads/page.test.tsx
git commit -m "feat(frontend): /admin/uploads grid skeleton

Standalone admin page that fetches /api/images, renders a thumbnail
grid with ref_count badges, and shows a totals header. Filters, sort,
detail rail, delete flow, and dropzone arrive in subsequent commits.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 9: Filters, sort, detail rail, Copy URL

**Files:**
- Modify: `frontend/src/app/admin/uploads/page.tsx`
- Modify: `frontend/src/app/admin/uploads/page.test.tsx`

Adds the left-rail filter (all / unused only) and sort (newest / name / size), plus a populated detail rail with preview, metadata, refs list, and Copy URL button.

- [ ] **Step 1: Write the failing tests**

Append to `frontend/src/app/admin/uploads/page.test.tsx`:

```tsx
import { fireEvent } from "@testing-library/react";

describe("UploadsPage — filters, sort, detail rail", () => {
  beforeEach(() => {
    localStorage.setItem("access_token", "tok");
  });

  it("'unused only' hides referenced items", async () => {
    installFetchMock(sample);
    render(<UploadsPage />);
    await waitFor(() => screen.getByText("1700000000_a.png"));

    fireEvent.click(screen.getByTestId("filter-unused"));

    expect(screen.queryByText("1700000000_a.png")).toBeNull();
    expect(screen.getByText("1700000001_b.png")).toBeDefined();
  });

  it("sorting by size puts the biggest first", async () => {
    installFetchMock(sample); // a=1024, b=2048
    render(<UploadsPage />);
    await waitFor(() => screen.getByText("1700000000_a.png"));

    fireEvent.click(screen.getByTestId("sort-size"));

    const tiles = screen.getAllByTestId(/^ref-badge-/);
    expect(tiles[0].getAttribute("data-testid")).toBe("ref-badge-1700000001_b.png");
  });

  it("clicking a thumb populates the detail rail", async () => {
    installFetchMock(sample, { "1700000000_a.png": ["blog/post"] });
    render(<UploadsPage />);
    await waitFor(() => screen.getByText("1700000000_a.png"));

    fireEvent.click(screen.getByText("1700000000_a.png").parentElement!);

    await waitFor(() => {
      expect(screen.getByTestId("detail-filename").textContent).toBe("1700000000_a.png");
      expect(screen.getByTestId("detail-size").textContent).toContain("1.0 KB");
    });
  });

  it("Copy URL writes the public URL to the clipboard", async () => {
    installFetchMock(sample);
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, "clipboard", {
      value: { writeText },
      configurable: true,
    });
    render(<UploadsPage />);
    await waitFor(() => screen.getByText("1700000000_a.png"));

    fireEvent.click(screen.getByText("1700000000_a.png").parentElement!);
    fireEvent.click(screen.getByTestId("copy-url"));

    expect(writeText).toHaveBeenCalledWith("/uploads/1700000000_a.png");
  });
});
```

- [ ] **Step 2: Run the tests to confirm they fail**

```bash
cd frontend && npm test -- page.test
```

Expected: the four new tests FAIL — selectors `filter-unused`, `sort-size`, `detail-filename`, `copy-url` don't exist yet.

- [ ] **Step 3: Implement filters, sort, and the detail rail**

Replace `frontend/src/app/admin/uploads/page.tsx` so it reads as below. (This is a full rewrite of the file from Task 8 — the new bits are the `view` / `sort` state, `visible` derivation, the left-rail controls, and the populated detail rail.)

```tsx
"use client";

import { useEffect, useMemo, useState } from "react";
import { apiFetch } from "@/lib/api";
import { logout } from "@/lib/auth";
import { ImageInfo, ImageRefs } from "@/lib/types";

type View = "all" | "unused";
type Sort = "newest" | "name" | "size";

export default function UploadsPage() {
  const [images, setImages] = useState<ImageInfo[]>([]);
  const [view, setView] = useState<View>("all");
  const [sort, setSort] = useState<Sort>("newest");
  const [selected, setSelected] = useState<string | null>(null);
  const [refs, setRefs] = useState<string[] | null>(null);
  const [loaded, setLoaded] = useState(false);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      const res = await apiFetch("/api/images");
      if (!res.ok) { setLoaded(true); return; }
      const data: ImageInfo[] = await res.json();
      if (!cancelled) { setImages(data); setLoaded(true); }
    })();
    return () => { cancelled = true; };
  }, []);

  // Load refs whenever the selected image changes.
  useEffect(() => {
    if (!selected) { setRefs(null); return; }
    let cancelled = false;
    (async () => {
      const res = await apiFetch(`/api/images/${encodeURIComponent(selected)}/refs`);
      if (!res.ok) { if (!cancelled) setRefs([]); return; }
      const data: ImageRefs = await res.json();
      if (!cancelled) setRefs(data.pages);
    })();
    return () => { cancelled = true; };
  }, [selected]);

  const visible = useMemo(() => {
    let arr = view === "unused" ? images.filter((i) => i.ref_count === 0) : images.slice();
    arr.sort((a, b) => {
      if (sort === "name") return a.filename.localeCompare(b.filename);
      if (sort === "size") return b.size - a.size;
      // newest = uploaded_at desc
      return b.uploaded_at.localeCompare(a.uploaded_at);
    });
    return arr;
  }, [images, view, sort]);

  const totalBytes = images.reduce((n, im) => n + im.size, 0);
  const unusedCount = images.filter((im) => im.ref_count === 0).length;
  const unusedBytes = images
    .filter((im) => im.ref_count === 0)
    .reduce((n, im) => n + im.size, 0);

  const selectedInfo = images.find((im) => im.filename === selected) ?? null;

  const copyUrl = async () => {
    if (!selectedInfo) return;
    await navigator.clipboard?.writeText(selectedInfo.url);
  };

  return (
    <div style={{ display: "flex", height: "100vh", overflow: "hidden", color: "var(--color)" }}>
      {/* Left rail */}
      <div style={leftRailStyle}>
        <div style={headerStyle}>
          <span style={{ color: "var(--accent)", fontWeight: "bold", fontSize: "0.9rem" }}>
            uploads
          </span>
          <span style={{ display: "flex", gap: 10 }}>
            <a href="/" style={navLinkStyle}>site</a>
            <a href="/admin/editor" style={navLinkStyle}>editor</a>
            <button onClick={logout} style={{ ...navLinkStyle, background: "none", border: "none", cursor: "pointer" }}>
              logout
            </button>
          </span>
        </div>

        <div style={sectionLabel}>view</div>
        <RailRow active={view === "all"} onClick={() => setView("all")} testid="filter-all">
          ★ all images <span style={{ float: "right", color: "rgba(255,255,255,0.4)" }}>{images.length}</span>
        </RailRow>
        <RailRow active={view === "unused"} onClick={() => setView("unused")} testid="filter-unused">
          unused only <span style={{ float: "right", color: "#ff6b6b" }}>{unusedCount}</span>
        </RailRow>

        <div style={sectionLabel}>sort</div>
        <RailRow active={sort === "newest"} onClick={() => setSort("newest")} testid="sort-newest">↓ newest</RailRow>
        <RailRow active={sort === "name"} onClick={() => setSort("name")} testid="sort-name">name</RailRow>
        <RailRow active={sort === "size"} onClick={() => setSort("size")} testid="sort-size">size</RailRow>
      </div>

      {/* Grid + totals */}
      <div style={{ flex: 1, padding: 14, overflow: "auto" }}>
        <div data-testid="totals" style={totalsStyle}>
          {loaded
            ? `${images.length} images · ${formatBytes(totalBytes)} total · ${unusedCount} unused (${formatBytes(unusedBytes)})`
            : "loading…"}
        </div>
        <div style={gridStyle}>
          {visible.map((im) => (
            <div
              key={im.filename}
              onClick={() => setSelected(im.filename)}
              style={{
                position: "relative",
                cursor: "pointer",
                outline: selected === im.filename ? "2px solid var(--accent)" : "none",
              }}
            >
              <img
                src={im.url}
                alt={im.filename}
                loading="lazy"
                style={{
                  width: "100%",
                  aspectRatio: "1",
                  objectFit: "cover",
                  background: "#1a1a1a",
                  display: "block",
                }}
              />
              <div
                data-testid={`ref-badge-${im.filename}`}
                title={refTitle(im.ref_count)}
                style={badgeStyle(im.ref_count)}
              >
                {refLabel(im.ref_count)}
              </div>
              <div style={filenameStyle}>{im.filename}</div>
            </div>
          ))}
        </div>
      </div>

      {/* Detail rail */}
      <div style={detailRailStyle}>
        {selectedInfo ? (
          <>
            <div style={sectionLabel}>selected</div>
            <img
              src={selectedInfo.url}
              alt={selectedInfo.filename}
              style={{ width: "100%", aspectRatio: "1", objectFit: "cover", background: "#1a1a1a", marginBottom: 10 }}
            />
            <div data-testid="detail-filename" style={{ fontSize: 11, wordBreak: "break-all", marginBottom: 4 }}>
              {selectedInfo.filename}
            </div>
            <div data-testid="detail-size" style={{ fontSize: 10, color: "rgba(255,255,255,0.4)", marginBottom: 14 }}>
              {formatBytes(selectedInfo.size)} · {selectedInfo.uploaded_at.slice(0, 10)}
            </div>

            <div style={sectionLabel}>used in ({refs?.length ?? "…"})</div>
            <div style={{ marginBottom: 18 }}>
              {refs?.length === 0 && (
                <div style={{ fontSize: 11, color: "rgba(255,255,255,0.4)" }}>not referenced</div>
              )}
              {refs?.map((page) => (
                <a
                  key={page}
                  href={`/admin/editor?path=${encodeURIComponent(page)}`}
                  style={{ display: "block", color: "rgba(255,255,255,0.7)", textDecoration: "none", fontSize: 11, padding: "3px 0" }}
                >
                  → /{page}
                </a>
              ))}
            </div>

            <div style={{ display: "flex", gap: 6 }}>
              <button data-testid="copy-url" onClick={copyUrl} style={{ ...primaryButton, flex: 1 }}>Copy URL</button>
            </div>
          </>
        ) : (
          <div style={{ fontSize: 11, color: "rgba(255,255,255,0.3)" }}>Select an image</div>
        )}
      </div>
    </div>
  );
}

function RailRow({
  active,
  onClick,
  testid,
  children,
}: {
  active: boolean;
  onClick: () => void;
  testid?: string;
  children: React.ReactNode;
}) {
  return (
    <div
      data-testid={testid}
      onClick={onClick}
      style={{
        cursor: "pointer",
        padding: "3px 8px",
        borderRadius: 3,
        marginBottom: 3,
        background: active ? "rgba(51,172,183,0.1)" : "transparent",
        color: active ? "var(--accent)" : "rgba(255,255,255,0.7)",
        fontSize: 12,
      }}
    >
      {children}
    </div>
  );
}

function refLabel(count: number): string {
  if (count < 0) return "?";
  return String(count);
}
function refTitle(count: number): string {
  if (count < 0) return "Couldn't compute references";
  if (count === 0) return "Not referenced anywhere";
  return `Used in ${count} page${count === 1 ? "" : "s"}`;
}
function badgeStyle(count: number): React.CSSProperties {
  let bg = "var(--accent)";
  if (count === 0) bg = "#ff6b6b";
  if (count < 0) bg = "rgba(255,255,255,0.3)";
  return { position: "absolute", top: 4, right: 4, background: bg, color: "#000", fontSize: 10, padding: "1px 5px", borderRadius: 2 };
}
function formatBytes(n: number): string {
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
  return `${(n / 1024 / 1024).toFixed(1)} MB`;
}

const leftRailStyle: React.CSSProperties = {
  width: 170,
  borderRight: "1px solid rgba(255,255,255,0.08)",
  padding: 14,
  flexShrink: 0,
  overflow: "auto",
};
const headerStyle: React.CSSProperties = {
  display: "flex",
  justifyContent: "space-between",
  alignItems: "center",
  marginBottom: 16,
};
const navLinkStyle: React.CSSProperties = {
  color: "rgba(255,255,255,0.4)",
  textDecoration: "none",
  fontFamily: "inherit",
  fontSize: "0.75rem",
};
const sectionLabel: React.CSSProperties = {
  fontSize: 10,
  color: "rgba(255,255,255,0.45)",
  textTransform: "uppercase",
  letterSpacing: "0.05em",
  margin: "14px 0 6px",
};
const totalsStyle: React.CSSProperties = {
  fontSize: 11,
  color: "rgba(255,255,255,0.5)",
  marginBottom: 10,
};
const gridStyle: React.CSSProperties = {
  display: "grid",
  gridTemplateColumns: "repeat(auto-fill, minmax(120px, 1fr))",
  gap: 8,
};
const filenameStyle: React.CSSProperties = {
  fontSize: 10,
  color: "rgba(255,255,255,0.45)",
  marginTop: 3,
  whiteSpace: "nowrap",
  overflow: "hidden",
  textOverflow: "ellipsis",
};
const detailRailStyle: React.CSSProperties = {
  width: 230,
  borderLeft: "1px solid rgba(255,255,255,0.08)",
  padding: 14,
  flexShrink: 0,
  overflow: "auto",
};
const primaryButton: React.CSSProperties = {
  background: "var(--accent)",
  border: "none",
  borderRadius: 4,
  padding: "6px 10px",
  color: "#000",
  cursor: "pointer",
  fontSize: 11,
  fontWeight: "bold",
};
```

The detail-rail "used in" list uses anchor `href` (not router navigation) so navigation goes through the existing static-export path. The editor's `?path=` integration from Task 7 picks up the page on mount.

- [ ] **Step 4: Update the existing `installFetchMock` so it also handles the per-image refs endpoint**

Edit the helper at the top of `frontend/src/app/admin/uploads/page.test.tsx`:

```tsx
function installFetchMock(initial: unknown, refsByFile: Record<string, string[]> = {}) {
  const fetchMock = vi.fn().mockImplementation((path: string) => {
    if (path === "/api/images") {
      return Promise.resolve(new Response(JSON.stringify(initial), { status: 200 }));
    }
    const m = path.match(/^\/api\/images\/(.+)\/refs$/);
    if (m) {
      const filename = decodeURIComponent(m[1]);
      const pages = refsByFile[filename] ?? [];
      return Promise.resolve(new Response(JSON.stringify({ filename, pages }), { status: 200 }));
    }
    return Promise.resolve(new Response("", { status: 404 }));
  });
  vi.stubGlobal("fetch", fetchMock);
  return fetchMock;
}
```

This keeps the Task 8 tests valid (the new arg defaults to `{}`).

- [ ] **Step 5: Run the tests to verify everything passes**

```bash
cd frontend && npm test -- page.test
```

Expected: all tests in `UploadsPage — grid skeleton` AND `UploadsPage — filters, sort, detail rail` PASS.

- [ ] **Step 6: Type-check**

```bash
cd frontend && npx tsc --noEmit
```

Expected: clean.

- [ ] **Step 7: Commit**

```bash
git add frontend/src/app/admin/uploads/page.tsx frontend/src/app/admin/uploads/page.test.tsx
git commit -m "feat(frontend): uploads filters, sort, detail rail, copy URL

Left-rail toggles for view (all / unused) and sort (newest / name /
size). Detail rail shows preview, filename, size, upload date, and the
clickable refs list pulled from /api/images/{filename}/refs. Copy URL
writes /uploads/<filename> to the clipboard.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 10: Delete flow (with confirm modal) and dropzone upload

**Files:**
- Modify: `frontend/src/app/admin/uploads/page.tsx`
- Modify: `frontend/src/app/admin/uploads/page.test.tsx`

Final UI piece. Delete on an unused item issues `DELETE /api/images/{filename}`; on a referenced item (or any 409 response) it opens a confirm modal listing the affected pages and re-issues with `?force=1`. The dropzone in the left rail accepts dragged files and a click-to-pick fallback, posting each to `POST /api/images` and prepending the result to the grid.

- [ ] **Step 1: Write the failing tests**

Append to `frontend/src/app/admin/uploads/page.test.tsx`:

```tsx
describe("UploadsPage — delete flow", () => {
  beforeEach(() => {
    localStorage.setItem("access_token", "tok");
  });

  it("deletes an unused image without confirmation", async () => {
    let deleteCalls = 0;
    const fetchMock = vi.fn().mockImplementation((path: string, init?: RequestInit) => {
      if (path === "/api/images" && (!init || init.method === undefined)) {
        return Promise.resolve(new Response(JSON.stringify(sample), { status: 200 }));
      }
      if (path === "/api/images/1700000001_b.png" && init?.method === "DELETE") {
        deleteCalls++;
        return Promise.resolve(new Response("", { status: 204 }));
      }
      const m = path.match(/^\/api\/images\/(.+)\/refs$/);
      if (m) return Promise.resolve(new Response(JSON.stringify({ filename: m[1], pages: [] }), { status: 200 }));
      return Promise.resolve(new Response("", { status: 404 }));
    });
    vi.stubGlobal("fetch", fetchMock);

    render(<UploadsPage />);
    await waitFor(() => screen.getByText("1700000001_b.png"));

    fireEvent.click(screen.getByText("1700000001_b.png").parentElement!);
    fireEvent.click(await screen.findByTestId("delete-button"));

    await waitFor(() => {
      expect(deleteCalls).toBe(1);
      expect(screen.queryByText("1700000001_b.png")).toBeNull();
    });
  });

  it("opens confirm modal for a referenced image and force-deletes on confirm", async () => {
    const calls: string[] = [];
    const fetchMock = vi.fn().mockImplementation((path: string, init?: RequestInit) => {
      calls.push(`${init?.method ?? "GET"} ${path}`);
      if (path === "/api/images" && (!init || init.method === undefined)) {
        return Promise.resolve(new Response(JSON.stringify(sample), { status: 200 }));
      }
      const m = path.match(/^\/api\/images\/(.+)\/refs$/);
      if (m) {
        const fname = decodeURIComponent(m[1]);
        const pages = fname === "1700000000_a.png" ? ["blog/post", "about"] : [];
        return Promise.resolve(new Response(JSON.stringify({ filename: fname, pages }), { status: 200 }));
      }
      if (path === "/api/images/1700000000_a.png?force=1" && init?.method === "DELETE") {
        return Promise.resolve(new Response("", { status: 204 }));
      }
      return Promise.resolve(new Response("", { status: 404 }));
    });
    vi.stubGlobal("fetch", fetchMock);

    render(<UploadsPage />);
    await waitFor(() => screen.getByText("1700000000_a.png"));
    fireEvent.click(screen.getByText("1700000000_a.png").parentElement!);
    fireEvent.click(await screen.findByTestId("delete-button"));

    // Confirm modal lists the pages.
    expect(await screen.findByTestId("confirm-modal")).toBeDefined();
    expect(screen.getByText("/blog/post")).toBeDefined();
    expect(screen.getByText("/about")).toBeDefined();

    fireEvent.click(screen.getByTestId("confirm-delete"));

    await waitFor(() => {
      expect(calls).toContain("DELETE /api/images/1700000000_a.png?force=1");
      expect(screen.queryByText("1700000000_a.png")).toBeNull();
    });
  });

  it("opens confirm modal when DELETE returns 409 for an item the grid showed as unused", async () => {
    const fetchMock = vi.fn().mockImplementation((path: string, init?: RequestInit) => {
      if (path === "/api/images" && (!init || init.method === undefined)) {
        return Promise.resolve(new Response(JSON.stringify(sample), { status: 200 }));
      }
      const m = path.match(/^\/api\/images\/(.+)\/refs$/);
      if (m) {
        const fname = decodeURIComponent(m[1]);
        return Promise.resolve(new Response(JSON.stringify({ filename: fname, pages: [] }), { status: 200 }));
      }
      if (path === "/api/images/1700000001_b.png" && init?.method === "DELETE") {
        return Promise.resolve(
          new Response(JSON.stringify({ error: "in use", pages: ["unexpected/page"] }), { status: 409 }),
        );
      }
      return Promise.resolve(new Response("", { status: 404 }));
    });
    vi.stubGlobal("fetch", fetchMock);

    render(<UploadsPage />);
    await waitFor(() => screen.getByText("1700000001_b.png"));
    fireEvent.click(screen.getByText("1700000001_b.png").parentElement!);
    fireEvent.click(await screen.findByTestId("delete-button"));

    expect(await screen.findByTestId("confirm-modal")).toBeDefined();
    expect(screen.getByText("/unexpected/page")).toBeDefined();
  });
});

describe("UploadsPage — upload", () => {
  beforeEach(() => {
    localStorage.setItem("access_token", "tok");
  });

  it("dropping a file POSTs to /api/images and prepends the response", async () => {
    const newInfo = {
      filename: "1700000999_c.png",
      url: "/uploads/1700000999_c.png",
      size: 100,
      ref_count: 0,
      uploaded_at: "2023-11-14T22:14:59Z",
    };
    const fetchMock = vi.fn().mockImplementation((path: string, init?: RequestInit) => {
      if (path === "/api/images" && (!init || init.method === undefined)) {
        return Promise.resolve(new Response(JSON.stringify(sample), { status: 200 }));
      }
      if (path === "/api/images" && init?.method === "POST") {
        return Promise.resolve(new Response(JSON.stringify(newInfo), { status: 201 }));
      }
      const m = path.match(/^\/api\/images\/(.+)\/refs$/);
      if (m) return Promise.resolve(new Response(JSON.stringify({ filename: m[1], pages: [] }), { status: 200 }));
      return Promise.resolve(new Response("", { status: 404 }));
    });
    vi.stubGlobal("fetch", fetchMock);

    render(<UploadsPage />);
    await waitFor(() => screen.getByText("1700000000_a.png"));

    const dropzone = screen.getByTestId("dropzone");
    const file = new File(["hello"], "c.png", { type: "image/png" });
    const dataTransfer = { files: [file], items: [], types: ["Files"] };
    fireEvent.drop(dropzone, { dataTransfer });

    await waitFor(() => {
      expect(screen.getByText("1700000999_c.png")).toBeDefined();
    });
  });
});
```

- [ ] **Step 2: Run the tests to confirm they fail**

```bash
cd frontend && npm test -- page.test
```

Expected: the new tests FAIL (`delete-button`, `confirm-modal`, `confirm-delete`, `dropzone` are not in the DOM yet).

- [ ] **Step 3: Wire delete + dropzone**

Edit `frontend/src/app/admin/uploads/page.tsx`. Make these focused changes (do **not** rewrite the file from scratch — patch in the marked spots):

1. Add `pendingDelete` state next to the other state hooks:

   ```tsx
   const [pendingDelete, setPendingDelete] = useState<{ filename: string; pages: string[] } | null>(null);
   ```

2. Add the delete handler below `copyUrl`:

   ```tsx
   const requestDelete = async (filename: string) => {
     // Use grid info to short-circuit the obvious "no refs" path.
     const info = images.find((i) => i.filename === filename);
     if (info && info.ref_count === 0) {
       const res = await apiFetch(`/api/images/${encodeURIComponent(filename)}`, { method: "DELETE" });
       if (res.status === 204) {
         setImages((prev) => prev.filter((i) => i.filename !== filename));
         setSelected((s) => (s === filename ? null : s));
         return;
       }
       if (res.status === 409) {
         const body = await res.json().catch(() => ({ pages: [] as string[] }));
         setPendingDelete({ filename, pages: body.pages ?? [] });
         return;
       }
       if (res.status === 404) {
         setImages((prev) => prev.filter((i) => i.filename !== filename));
         setSelected((s) => (s === filename ? null : s));
         return;
       }
       return;
     }

     // Has refs (or unknown). Always confirm.
     const refsRes = await apiFetch(`/api/images/${encodeURIComponent(filename)}/refs`);
     const pages = refsRes.ok ? ((await refsRes.json()) as ImageRefs).pages : [];
     setPendingDelete({ filename, pages });
   };

   const confirmForceDelete = async () => {
     if (!pendingDelete) return;
     const res = await apiFetch(
       `/api/images/${encodeURIComponent(pendingDelete.filename)}?force=1`,
       { method: "DELETE" },
     );
     if (res.status === 204 || res.status === 404) {
       setImages((prev) => prev.filter((i) => i.filename !== pendingDelete.filename));
       setSelected((s) => (s === pendingDelete.filename ? null : s));
     }
     setPendingDelete(null);
   };
   ```

3. In the detail-rail buttons row, add a Delete button:

   ```tsx
   <div style={{ display: "flex", gap: 6 }}>
     <button data-testid="copy-url" onClick={copyUrl} style={{ ...primaryButton, flex: 1 }}>Copy URL</button>
     <button
       data-testid="delete-button"
       onClick={() => selectedInfo && requestDelete(selectedInfo.filename)}
       style={{
         flex: 1,
         background: "none",
         border: "1px solid rgba(255,100,100,0.4)",
         color: "#ff6b6b",
         borderRadius: 4,
         padding: "6px 10px",
         cursor: "pointer",
         fontSize: 11,
       }}
     >
       Delete
     </button>
   </div>
   ```

4. Above the closing `</div>` of the outermost flex container, render the confirm modal:

   ```tsx
   {pendingDelete && (
     <div data-testid="confirm-modal" style={modalBackdrop}>
       <div style={modalBoxStyle}>
         <div style={{ fontSize: 13, marginBottom: 10 }}>
           Delete <code>{pendingDelete.filename}</code>?
         </div>
         <div style={{ fontSize: 12, color: "rgba(255,255,255,0.6)", marginBottom: 8 }}>
           Still referenced by:
         </div>
         <div style={{ marginBottom: 14, maxHeight: 160, overflow: "auto" }}>
           {pendingDelete.pages.map((p) => (
             <a
               key={p}
               href={`/admin/editor?path=${encodeURIComponent(p)}`}
               style={{ display: "block", color: "rgba(255,255,255,0.85)", textDecoration: "none", fontSize: 11, padding: "3px 0" }}
             >
               /{p}
             </a>
           ))}
         </div>
         <div style={{ display: "flex", gap: 8, justifyContent: "flex-end" }}>
           <button onClick={() => setPendingDelete(null)} style={cancelButton}>Cancel</button>
           <button data-testid="confirm-delete" onClick={confirmForceDelete} style={dangerButton}>Delete anyway</button>
         </div>
       </div>
     </div>
   )}
   ```

5. Add the dropzone to the left rail, just above the closing `</div>` of `leftRailStyle`:

   ```tsx
   <div
     data-testid="dropzone"
     onDragOver={(e) => { e.preventDefault(); }}
     onDrop={(e) => {
       e.preventDefault();
       const files = Array.from(e.dataTransfer?.files ?? []);
       files.forEach((f) => uploadFile(f));
     }}
     onClick={() => document.getElementById("upload-input")?.click()}
     style={dropzoneStyle}
   >
     ⬆ drag &amp; drop<br />or click to upload
     <input
       id="upload-input"
       type="file"
       accept="image/*"
       multiple
       style={{ display: "none" }}
       onChange={(e) => {
         const files = Array.from(e.target.files ?? []);
         files.forEach((f) => uploadFile(f));
         e.currentTarget.value = "";
       }}
     />
   </div>
   ```

6. Add the upload helper next to `requestDelete`:

   ```tsx
   const uploadFile = async (file: File) => {
     const form = new FormData();
     form.append("file", file);
     const res = await apiFetch("/api/images", { method: "POST", body: form });
     if (res.ok) {
       const info: ImageInfo = await res.json();
       setImages((prev) => [info, ...prev.filter((i) => i.filename !== info.filename)]);
     }
   };
   ```

7. Add the new style consts at the bottom:

   ```tsx
   const dropzoneStyle: React.CSSProperties = {
     marginTop: 18,
     border: "1px dashed rgba(255,255,255,0.2)",
     borderRadius: 4,
     padding: 14,
     textAlign: "center",
     color: "rgba(255,255,255,0.5)",
     fontSize: 11,
     cursor: "pointer",
     background: "rgba(255,255,255,0.02)",
   };
   const modalBackdrop: React.CSSProperties = {
     position: "fixed", inset: 0, background: "rgba(0,0,0,0.55)",
     display: "flex", alignItems: "center", justifyContent: "center", zIndex: 9999,
   };
   const modalBoxStyle: React.CSSProperties = {
     background: "var(--background)",
     border: "1px solid rgba(255,255,255,0.15)",
     borderRadius: 6,
     padding: 18,
     width: 380,
   };
   const cancelButton: React.CSSProperties = {
     background: "none",
     border: "1px solid rgba(255,255,255,0.15)",
     color: "rgba(255,255,255,0.7)",
     padding: "5px 12px",
     borderRadius: 4,
     cursor: "pointer",
     fontSize: 11,
   };
   const dangerButton: React.CSSProperties = {
     background: "#ff6b6b",
     border: "none",
     color: "#000",
     padding: "5px 12px",
     borderRadius: 4,
     cursor: "pointer",
     fontSize: 11,
     fontWeight: "bold",
   };
   ```

- [ ] **Step 4: Run the tests to verify they pass**

```bash
cd frontend && npm test -- page.test
```

Expected: every test in `page.test.tsx` passes.

- [ ] **Step 5: Type-check the frontend**

```bash
cd frontend && npx tsc --noEmit
```

Expected: clean.

- [ ] **Step 6: Run the full Go suite as a regression check**

```bash
go test ./...
```

Expected: all PASS.

- [ ] **Step 7: Smoke-test the page in the browser**

In one terminal:

```bash
cd frontend && npm run dev
```

In another:

```bash
go run ./cmd/server
```

Open `http://localhost:3000/admin/uploads` (after logging in via `/admin/login`). Verify:
- The grid lists every file in `uploads/` with the right ref-count badges.
- Filter switches between all / unused.
- Sort changes order.
- Clicking a thumb populates the rail and shows the right "Used in" list.
- Copy URL writes the public URL.
- Deleting an unused image removes it.
- Deleting a referenced image opens the confirm modal with the page list; clicking "Delete anyway" force-deletes.
- Drag-and-drop uploading appends new tiles to the grid.
- The editor's "uploads" header link works, and clicking a "Used in" entry opens the editor with that page selected.

If anything misbehaves, fix it before committing.

- [ ] **Step 8: Commit**

```bash
git add frontend/src/app/admin/uploads/page.tsx frontend/src/app/admin/uploads/page.test.tsx
git commit -m "feat(frontend): uploads delete flow + dropzone

Delete on unused → DELETE /api/images/{filename}. Delete on referenced
(or 409 race) → confirm modal listing affected pages, then ?force=1.
Left-rail dropzone + click-to-pick uploads via POST /api/images and
prepends the response.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Final verification

- [ ] **Step 1: Run the full test suite**

```bash
make test
```

Expected: Go and frontend tests both pass.

- [ ] **Step 2: Build to make sure the static export still works**

```bash
make build
```

Expected: builds without errors.

- [ ] **Step 3: Confirm the working tree is clean**

```bash
git status
```

Expected: only the spec file under `docs/superpowers/specs/` and `docs/superpowers/plans/` exist as untracked-vs-history; the implementation commits all landed.

---

## Self-Review Notes

- All spec sections map to a task: ImageInfo extension (T1), Refs (T2), Delete force/InUseError (T3), List enrichment (T4), GetRefs route (T5), Delete handler 409/force (T6), types + editor link + ?path= (T7), grid skeleton (T8), filters + sort + detail rail + Copy URL (T9), delete flow + dropzone (T10).
- No placeholders. Every code block is the actual code to write.
- Type consistency: `Service.List(refs)`, `Service.Delete(filename, force, pages)`, `Service.Refs(contentDir)`, `Handler.GetRefs`, `NewHandler(svc, contentDir)` all introduced together and used consistently across tasks.
- Test convention: co-located `*.test.ts(x)` matches the existing `frontend/src/lib/*.test.*` files. The spec's hint at `__tests__/` is not the actual repo convention.
