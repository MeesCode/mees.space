package images

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func createTestImage(t *testing.T) (*bytes.Buffer, string) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	img.Set(0, 0, color.RGBA{255, 0, 0, 255})

	var buf bytes.Buffer
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "test.png")
	png.Encode(&buf, img)
	part.Write(buf.Bytes())
	writer.Close()

	return body, writer.FormDataContentType()
}

func TestUploadImage(t *testing.T) {
	uploadsDir := t.TempDir()
	svc := NewService(uploadsDir)
	h := NewHandler(svc, "")

	body, contentType := createTestImage(t)

	req := httptest.NewRequest("POST", "/api/images", body)
	req.Header.Set("Content-Type", contentType)
	rr := httptest.NewRecorder()

	h.Upload(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	var info ImageInfo
	json.NewDecoder(rr.Body).Decode(&info)
	if info.Filename == "" {
		t.Fatal("expected filename")
	}

	// Verify file exists on disk
	if _, err := os.Stat(filepath.Join(uploadsDir, info.Filename)); err != nil {
		t.Fatal("uploaded file not found on disk")
	}
}

func TestUploadInvalidType(t *testing.T) {
	uploadsDir := t.TempDir()
	svc := NewService(uploadsDir)
	h := NewHandler(svc, "")

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "test.txt")
	part.Write([]byte("this is not an image"))
	writer.Close()

	req := httptest.NewRequest("POST", "/api/images", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rr := httptest.NewRecorder()

	h.Upload(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestListImages(t *testing.T) {
	uploadsDir := t.TempDir()
	svc := NewService(uploadsDir)
	h := NewHandler(svc, "")

	// Create a test file
	os.WriteFile(filepath.Join(uploadsDir, "test.png"), []byte("fake"), 0644)

	req := httptest.NewRequest("GET", "/api/images", nil)
	rr := httptest.NewRecorder()

	h.List(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var images []ImageInfo
	json.NewDecoder(rr.Body).Decode(&images)
	if len(images) != 1 {
		t.Errorf("expected 1 image, got %d", len(images))
	}
}

func TestDeleteImage(t *testing.T) {
	uploadsDir := t.TempDir()
	svc := NewService(uploadsDir)
	h := NewHandler(svc, "")

	os.WriteFile(filepath.Join(uploadsDir, "test.png"), []byte("fake"), 0644)

	req := httptest.NewRequest("DELETE", "/api/images/test.png", nil)
	req.SetPathValue("filename", "test.png")
	rr := httptest.NewRecorder()

	h.Delete(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d: %s", rr.Code, rr.Body.String())
	}

	if _, err := os.Stat(filepath.Join(uploadsDir, "test.png")); !os.IsNotExist(err) {
		t.Error("file should be deleted")
	}
}

func TestDeleteImage_NotFound(t *testing.T) {
	uploadsDir := t.TempDir()
	svc := NewService(uploadsDir)
	h := NewHandler(svc, "")

	req := httptest.NewRequest("DELETE", "/api/images/nonexistent.png", nil)
	req.SetPathValue("filename", "nonexistent.png")
	rr := httptest.NewRecorder()

	h.Delete(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rr.Code)
	}
}

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
