package folders

import (
	"database/sql"
	"encoding/json"
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
	db         *sql.DB
}

func NewHandler(contentDir string, db *sql.DB) *Handler {
	return &Handler{contentDir: contentDir, db: db}
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

	recursive := r.URL.Query().Get("recursive") == "true"

	if recursive {
		// Delete all page DB rows with paths under this folder
		h.db.Exec("DELETE FROM pages WHERE path LIKE ?", clean+"/%")

		if err := os.RemoveAll(fullPath); err != nil {
			http.Error(w, `{"error":"failed to delete folder"}`, http.StatusInternalServerError)
			return
		}
	} else {
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
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) Rename(w http.ResponseWriter, r *http.Request) {
	folderPath := r.PathValue("path")

	clean, err := sanitizePath(folderPath)
	if err != nil {
		http.Error(w, `{"error":"invalid path"}`, http.StatusBadRequest)
		return
	}

	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Name == "" {
		http.Error(w, `{"error":"name is required"}`, http.StatusBadRequest)
		return
	}

	// Validate the new name doesn't contain path separators or dots
	if strings.ContainsAny(body.Name, "/\\..") {
		http.Error(w, `{"error":"invalid name"}`, http.StatusBadRequest)
		return
	}

	oldFull := filepath.Join(h.contentDir, clean)
	if !isWithinDir(oldFull, h.contentDir) {
		http.Error(w, `{"error":"invalid path"}`, http.StatusBadRequest)
		return
	}

	info, err := os.Stat(oldFull)
	if os.IsNotExist(err) {
		http.Error(w, `{"error":"folder not found"}`, http.StatusNotFound)
		return
	}
	if !info.IsDir() {
		http.Error(w, `{"error":"not a folder"}`, http.StatusBadRequest)
		return
	}

	// Build new path: same parent directory, new name
	parentDir := filepath.Dir(clean)
	var newClean string
	if parentDir == "." {
		newClean = body.Name
	} else {
		newClean = parentDir + "/" + body.Name
	}

	newFull := filepath.Join(h.contentDir, newClean)
	if !isWithinDir(newFull, h.contentDir) {
		http.Error(w, `{"error":"invalid path"}`, http.StatusBadRequest)
		return
	}

	if _, err := os.Stat(newFull); err == nil {
		http.Error(w, `{"error":"target folder already exists"}`, http.StatusConflict)
		return
	}

	if err := os.Rename(oldFull, newFull); err != nil {
		http.Error(w, `{"error":"failed to rename folder"}`, http.StatusInternalServerError)
		return
	}

	// Update all page paths in DB
	oldPrefix := clean + "/"
	newPrefix := newClean + "/"
	h.db.Exec(
		"UPDATE pages SET path = ? || SUBSTR(path, ?) WHERE path LIKE ?",
		newPrefix, len(oldPrefix)+1, oldPrefix+"%",
	)

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"path":"%s"}`, newClean)
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
