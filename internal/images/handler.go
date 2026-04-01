package images

import (
	"encoding/json"
	"net/http"
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
		http.Error(w, `{"error":"file too large (max 10MB)"}`, http.StatusRequestEntityTooLarge)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, `{"error":"missing file field"}`, http.StatusBadRequest)
		return
	}
	defer file.Close()

	info, err := h.svc.Upload(file, header)
	if err == ErrInvalidType {
		http.Error(w, `{"error":"invalid file type, only images allowed"}`, http.StatusBadRequest)
		return
	}
	if err != nil {
		http.Error(w, `{"error":"failed to upload file"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(info)
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	images, err := h.svc.List()
	if err != nil {
		http.Error(w, `{"error":"failed to list images"}`, http.StatusInternalServerError)
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
		http.Error(w, `{"error":"filename required"}`, http.StatusBadRequest)
		return
	}

	err := h.svc.Delete(filename)
	if err == ErrNotFound {
		http.Error(w, `{"error":"image not found"}`, http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, `{"error":"failed to delete image"}`, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
