package images

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"mees.space/internal/httputil"
)

const maxUploadSize = 10 << 20 // 10MB

type Handler struct {
	svc        *Service
	contentDir string
}

func NewHandler(svc *Service, contentDir string) *Handler {
	return &Handler{svc: svc, contentDir: contentDir}
}

func (h *Handler) Upload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)

	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		httputil.JSONError(w, "file too large (max 10MB)", http.StatusRequestEntityTooLarge)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		httputil.JSONError(w, "missing file field", http.StatusBadRequest)
		return
	}
	defer file.Close()

	info, err := h.svc.Upload(file, header)
	if err == ErrInvalidType {
		httputil.JSONError(w, "invalid file type, only images allowed", http.StatusBadRequest)
		return
	}
	if err != nil {
		httputil.JSONError(w, "failed to upload file", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(info)
}

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
