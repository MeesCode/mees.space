package folders

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

var (
	ErrInvalidPath = errors.New("invalid path")
	ErrNotEmpty    = errors.New("folder not empty")
	ErrNotFound    = errors.New("folder not found")
)

type Handler struct {
	contentDir string
}

func NewHandler(contentDir string) *Handler {
	return &Handler{contentDir: contentDir}
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	folderPath := r.PathValue("path")

	clean, err := sanitizePath(folderPath)
	if err != nil {
		http.Error(w, `{"error":"invalid path"}`, http.StatusBadRequest)
		return
	}

	fullPath := filepath.Join(h.contentDir, clean)
	if !isWithinDir(fullPath, h.contentDir) {
		http.Error(w, `{"error":"invalid path"}`, http.StatusBadRequest)
		return
	}

	if err := os.MkdirAll(fullPath, 0755); err != nil {
		http.Error(w, `{"error":"failed to create folder"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	fmt.Fprintf(w, `{"path":"%s"}`, clean)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	folderPath := r.PathValue("path")

	clean, err := sanitizePath(folderPath)
	if err != nil {
		http.Error(w, `{"error":"invalid path"}`, http.StatusBadRequest)
		return
	}

	fullPath := filepath.Join(h.contentDir, clean)
	if !isWithinDir(fullPath, h.contentDir) {
		http.Error(w, `{"error":"invalid path"}`, http.StatusBadRequest)
		return
	}

	info, err := os.Stat(fullPath)
	if os.IsNotExist(err) {
		http.Error(w, `{"error":"folder not found"}`, http.StatusNotFound)
		return
	}
	if !info.IsDir() {
		http.Error(w, `{"error":"not a folder"}`, http.StatusBadRequest)
		return
	}

	entries, err := os.ReadDir(fullPath)
	if err != nil {
		http.Error(w, `{"error":"failed to read folder"}`, http.StatusInternalServerError)
		return
	}
	if len(entries) > 0 {
		http.Error(w, `{"error":"folder is not empty"}`, http.StatusBadRequest)
		return
	}

	if err := os.Remove(fullPath); err != nil {
		http.Error(w, `{"error":"failed to delete folder"}`, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func sanitizePath(raw string) (string, error) {
	if raw == "" {
		return "", ErrInvalidPath
	}

	// Reject paths that start with / (absolute on Unix)
	if strings.HasPrefix(raw, "/") || strings.HasPrefix(raw, "\\") {
		return "", ErrInvalidPath
	}

	cleaned := filepath.ToSlash(filepath.Clean(raw))
	cleaned = strings.TrimPrefix(cleaned, "/")

	if cleaned == "" || cleaned == "." {
		return "", ErrInvalidPath
	}

	if filepath.IsAbs(cleaned) || strings.Contains(cleaned, "..") {
		return "", ErrInvalidPath
	}

	for _, r := range cleaned {
		if r == 0 {
			return "", ErrInvalidPath
		}
	}

	return cleaned, nil
}

func isWithinDir(path, dir string) bool {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return false
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	return strings.HasPrefix(absPath, absDir+string(filepath.Separator)) || strings.HasPrefix(absPath, absDir+"/")
}
