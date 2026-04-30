package images

import (
	"encoding/json"
	"net/http"

	"mees.space/internal/httputil"
)

const maxUploadSize = 10 << 20 // 10MB

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
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
