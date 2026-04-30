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
