package folders

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"mees.space/internal/httputil"
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

	clean, err := httputil.SanitizePath(folderPath)
	if err != nil {
		httputil.JSONError(w, "invalid path", http.StatusBadRequest)
		return
	}

	fullPath := filepath.Join(h.contentDir, clean)
	if !httputil.IsWithinDir(fullPath, h.contentDir) {
		httputil.JSONError(w, "invalid path", http.StatusBadRequest)
		return
	}

	if err := os.MkdirAll(fullPath, 0755); err != nil {
		httputil.JSONError(w, "failed to create folder", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"path": clean})
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	folderPath := r.PathValue("path")

	clean, err := httputil.SanitizePath(folderPath)
	if err != nil {
		httputil.JSONError(w, "invalid path", http.StatusBadRequest)
		return
	}

	fullPath := filepath.Join(h.contentDir, clean)
	if !httputil.IsWithinDir(fullPath, h.contentDir) {
		httputil.JSONError(w, "invalid path", http.StatusBadRequest)
		return
	}

	info, err := os.Stat(fullPath)
	if os.IsNotExist(err) {
		httputil.JSONError(w, "folder not found", http.StatusNotFound)
		return
	}
	if !info.IsDir() {
		httputil.JSONError(w, "not a folder", http.StatusBadRequest)
		return
	}

	recursive := r.URL.Query().Get("recursive") == "true"

	if recursive {
		if _, err := h.db.Exec("DELETE FROM pages WHERE path LIKE ?", clean+"/%"); err != nil {
			httputil.JSONError(w, "failed to clean up page records", http.StatusInternalServerError)
			return
		}

		if err := os.RemoveAll(fullPath); err != nil {
			httputil.JSONError(w, "failed to delete folder", http.StatusInternalServerError)
			return
		}
	} else {
		entries, err := os.ReadDir(fullPath)
		if err != nil {
			httputil.JSONError(w, "failed to read folder", http.StatusInternalServerError)
			return
		}
		if len(entries) > 0 {
			httputil.JSONError(w, "folder is not empty", http.StatusBadRequest)
			return
		}

		if err := os.Remove(fullPath); err != nil {
			httputil.JSONError(w, "failed to delete folder", http.StatusInternalServerError)
			return
		}
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) Rename(w http.ResponseWriter, r *http.Request) {
	folderPath := r.PathValue("path")

	clean, err := httputil.SanitizePath(folderPath)
	if err != nil {
		httputil.JSONError(w, "invalid path", http.StatusBadRequest)
		return
	}

	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Name == "" {
		httputil.JSONError(w, "name is required", http.StatusBadRequest)
		return
	}

	if strings.ContainsAny(body.Name, "/\\..") {
		httputil.JSONError(w, "invalid name", http.StatusBadRequest)
		return
	}

	oldFull := filepath.Join(h.contentDir, clean)
	if !httputil.IsWithinDir(oldFull, h.contentDir) {
		httputil.JSONError(w, "invalid path", http.StatusBadRequest)
		return
	}

	info, err := os.Stat(oldFull)
	if os.IsNotExist(err) {
		httputil.JSONError(w, "folder not found", http.StatusNotFound)
		return
	}
	if !info.IsDir() {
		httputil.JSONError(w, "not a folder", http.StatusBadRequest)
		return
	}

	parentDir := filepath.Dir(clean)
	var newClean string
	if parentDir == "." {
		newClean = body.Name
	} else {
		newClean = parentDir + "/" + body.Name
	}

	newFull := filepath.Join(h.contentDir, newClean)
	if !httputil.IsWithinDir(newFull, h.contentDir) {
		httputil.JSONError(w, "invalid path", http.StatusBadRequest)
		return
	}

	if _, err := os.Stat(newFull); err == nil {
		httputil.JSONError(w, "target folder already exists", http.StatusConflict)
		return
	}

	if err := os.Rename(oldFull, newFull); err != nil {
		httputil.JSONError(w, "failed to rename folder", http.StatusInternalServerError)
		return
	}

	// Update all page paths in DB
	oldPrefix := clean + "/"
	newPrefix := newClean + "/"
	if _, err := h.db.Exec(
		"UPDATE pages SET path = ? || SUBSTR(path, ?) WHERE path LIKE ?",
		newPrefix, len(oldPrefix)+1, oldPrefix+"%",
	); err != nil {
		// Roll back file rename on DB failure
		os.Rename(newFull, oldFull)
		httputil.JSONError(w, "failed to update page paths", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"path":"%s"}`, newClean)
}
