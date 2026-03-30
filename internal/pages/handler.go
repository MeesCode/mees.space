package pages

import (
	"encoding/json"
	"net/http"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) GetTree(w http.ResponseWriter, r *http.Request) {
	tree, err := BuildContentTree(h.svc.db, h.svc.contentDir)
	if err != nil {
		http.Error(w, `{"error":"failed to build tree"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tree)
}

func (h *Handler) GetPage(w http.ResponseWriter, r *http.Request) {
	pagePath := extractPath(r)

	page, err := h.svc.GetPage(pagePath)
	if err == ErrNotFound {
		http.Error(w, `{"error":"page not found"}`, http.StatusNotFound)
		return
	}
	if err == ErrInvalidPath {
		http.Error(w, `{"error":"invalid path"}`, http.StatusBadRequest)
		return
	}
	if err != nil {
		http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(page)
}

func (h *Handler) CreatePage(w http.ResponseWriter, r *http.Request) {
	pagePath := extractPath(r)

	var req PageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	if req.Title == "" {
		http.Error(w, `{"error":"title is required"}`, http.StatusBadRequest)
		return
	}

	err := h.svc.CreatePage(pagePath, req.Title, req.Content)
	if err == ErrExists {
		http.Error(w, `{"error":"page already exists"}`, http.StatusConflict)
		return
	}
	if err == ErrInvalidPath {
		http.Error(w, `{"error":"invalid path"}`, http.StatusBadRequest)
		return
	}
	if err != nil {
		http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"path": pagePath, "title": req.Title})
}

func (h *Handler) UpdatePage(w http.ResponseWriter, r *http.Request) {
	pagePath := extractPath(r)

	var req PageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	if req.Title == "" {
		http.Error(w, `{"error":"title is required"}`, http.StatusBadRequest)
		return
	}

	err := h.svc.UpdatePage(pagePath, req.Title, req.Content)
	if err == ErrNotFound {
		http.Error(w, `{"error":"page not found"}`, http.StatusNotFound)
		return
	}
	if err == ErrInvalidPath {
		http.Error(w, `{"error":"invalid path"}`, http.StatusBadRequest)
		return
	}
	if err != nil {
		http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"path": pagePath, "title": req.Title})
}

func (h *Handler) DeletePage(w http.ResponseWriter, r *http.Request) {
	pagePath := extractPath(r)

	err := h.svc.DeletePage(pagePath)
	if err == ErrNotFound {
		http.Error(w, `{"error":"page not found"}`, http.StatusNotFound)
		return
	}
	if err == ErrInvalidPath {
		http.Error(w, `{"error":"invalid path"}`, http.StatusBadRequest)
		return
	}
	if err != nil {
		http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) IncrementView(w http.ResponseWriter, r *http.Request) {
	pagePath := r.PathValue("path")

	count, err := h.svc.IncrementViewCount(pagePath)
	if err == ErrNotFound {
		http.Error(w, `{"error":"page not found"}`, http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]int{"view_count": count})
}

func extractPath(r *http.Request) string {
	return r.PathValue("path")
}
