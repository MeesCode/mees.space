package folders

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestCreateFolder(t *testing.T) {
	contentDir := t.TempDir()
	h := NewHandler(contentDir, nil)

	req := httptest.NewRequest("POST", "/api/folders/blogs", nil)
	req.SetPathValue("path", "blogs")
	rr := httptest.NewRecorder()

	h.Create(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	info, err := os.Stat(filepath.Join(contentDir, "blogs"))
	if err != nil {
		t.Fatal("folder not created")
	}
	if !info.IsDir() {
		t.Fatal("expected directory")
	}
}

func TestCreateNestedFolder(t *testing.T) {
	contentDir := t.TempDir()
	h := NewHandler(contentDir, nil)

	req := httptest.NewRequest("POST", "/api/folders/recipes/easy", nil)
	req.SetPathValue("path", "recipes/easy")
	rr := httptest.NewRecorder()

	h.Create(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	info, err := os.Stat(filepath.Join(contentDir, "recipes", "easy"))
	if err != nil {
		t.Fatal("nested folder not created")
	}
	if !info.IsDir() {
		t.Fatal("expected directory")
	}
}

func TestDeleteEmptyFolder(t *testing.T) {
	contentDir := t.TempDir()
	os.MkdirAll(filepath.Join(contentDir, "empty-folder"), 0755)
	h := NewHandler(contentDir, nil)

	req := httptest.NewRequest("DELETE", "/api/folders/empty-folder", nil)
	req.SetPathValue("path", "empty-folder")
	rr := httptest.NewRecorder()

	h.Delete(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rr.Code, rr.Body.String())
	}

	if _, err := os.Stat(filepath.Join(contentDir, "empty-folder")); !os.IsNotExist(err) {
		t.Error("folder should be deleted")
	}
}

func TestDeleteNonEmptyFolder(t *testing.T) {
	contentDir := t.TempDir()
	os.MkdirAll(filepath.Join(contentDir, "has-files"), 0755)
	os.WriteFile(filepath.Join(contentDir, "has-files", "file.md"), []byte("content"), 0644)
	h := NewHandler(contentDir, nil)

	req := httptest.NewRequest("DELETE", "/api/folders/has-files", nil)
	req.SetPathValue("path", "has-files")
	rr := httptest.NewRecorder()

	h.Delete(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestDeleteNonexistentFolder(t *testing.T) {
	contentDir := t.TempDir()
	h := NewHandler(contentDir, nil)

	req := httptest.NewRequest("DELETE", "/api/folders/nonexistent", nil)
	req.SetPathValue("path", "nonexistent")
	rr := httptest.NewRecorder()

	h.Delete(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rr.Code)
	}
}

func TestPathTraversal(t *testing.T) {
	contentDir := t.TempDir()
	h := NewHandler(contentDir, nil)

	paths := []string{"../etc", "../../secret", "/absolute"}
	for _, p := range paths {
		req := httptest.NewRequest("POST", "/api/folders/"+p, nil)
		req.SetPathValue("path", p)
		rr := httptest.NewRecorder()

		h.Create(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("path %q: expected 400, got %d", p, rr.Code)
		}
	}
}
