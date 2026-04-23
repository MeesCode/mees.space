package pages

import (
	"encoding/json"
	"log"
	"net/http"

	"mees.space/internal/auth"
	"mees.space/internal/httputil"
)

type Handler struct {
	svc     *Service
	baseURL string
	descGen *Generator
}

func NewHandler(svc *Service, baseURL string, descGen *Generator) *Handler {
	return &Handler{svc: svc, baseURL: baseURL, descGen: descGen}
}

func (h *Handler) GetTree(w http.ResponseWriter, r *http.Request) {
	// Only include drafts if the request is authenticated
	isAuthed := auth.GetUser(r.Context()) != nil
	includeDrafts := isAuthed && r.URL.Query().Get("drafts") == "true"

	tree, err := BuildContentTree(h.svc.db, h.svc.contentDir, includeDrafts)
	if err != nil {
		httputil.JSONError(w, "failed to build tree", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tree)
}

func (h *Handler) GetPage(w http.ResponseWriter, r *http.Request) {
	pagePath := extractPath(r)

	page, err := h.svc.GetPage(pagePath)
	if err == ErrNotFound {
		httputil.JSONError(w, "page not found", http.StatusNotFound)
		return
	}
	if err == ErrInvalidPath {
		httputil.JSONError(w, "invalid path", http.StatusBadRequest)
		return
	}
	if err != nil {
		httputil.JSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	// Hide unpublished pages from unauthenticated users
	isAuthed := auth.GetUser(r.Context()) != nil
	if !page.Published && !isAuthed {
		httputil.JSONError(w, "page not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(page)
}

func (h *Handler) CreatePage(w http.ResponseWriter, r *http.Request) {
	pagePath := extractPath(r)

	var req PageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.JSONError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Title == "" {
		httputil.JSONError(w, "title is required", http.StatusBadRequest)
		return
	}

	err := h.svc.CreatePage(pagePath, req.Title, req.Content)
	if err == ErrExists {
		httputil.JSONError(w, "page already exists", http.StatusConflict)
		return
	}
	if err == ErrInvalidPath {
		httputil.JSONError(w, "invalid path", http.StatusBadRequest)
		return
	}
	if err != nil {
		httputil.JSONError(w, "internal server error", http.StatusInternalServerError)
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
		httputil.JSONError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Title == "" {
		httputil.JSONError(w, "title is required", http.StatusBadRequest)
		return
	}

	err := h.svc.UpdatePage(pagePath, req.Title, req.Content, req.ShowDate, req.Published, req.CreatedAt)
	if err == ErrNotFound {
		httputil.JSONError(w, "page not found", http.StatusNotFound)
		return
	}
	if err == ErrInvalidPath {
		httputil.JSONError(w, "invalid path", http.StatusBadRequest)
		return
	}
	if err != nil {
		httputil.JSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	// On manual save, regenerate description (synchronous, best-effort).
	if req.Manual != nil && *req.Manual && h.descGen != nil {
		desc := h.descGen.Generate(r.Context(), req.Title, req.Content)
		if _, dbErr := h.svc.db.Exec("UPDATE pages SET description = ? WHERE path = ?", desc, pagePath); dbErr != nil {
			log.Printf("description: write failed for %s: %v", pagePath, dbErr)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"path": pagePath, "title": req.Title})
}

func (h *Handler) RenamePage(w http.ResponseWriter, r *http.Request) {
	oldPath := extractPath(r)

	var req struct {
		NewPath string `json:"new_path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.JSONError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.NewPath == "" {
		httputil.JSONError(w, "new_path is required", http.StatusBadRequest)
		return
	}

	err := h.svc.RenamePage(oldPath, req.NewPath)
	if err == ErrNotFound {
		httputil.JSONError(w, "page not found", http.StatusNotFound)
		return
	}
	if err == ErrExists {
		httputil.JSONError(w, "a page already exists at that path", http.StatusConflict)
		return
	}
	if err == ErrInvalidPath {
		httputil.JSONError(w, "invalid path", http.StatusBadRequest)
		return
	}
	if err != nil {
		httputil.JSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"path": req.NewPath})
}

func (h *Handler) DeletePage(w http.ResponseWriter, r *http.Request) {
	pagePath := extractPath(r)

	err := h.svc.DeletePage(pagePath)
	if err == ErrNotFound {
		httputil.JSONError(w, "page not found", http.StatusNotFound)
		return
	}
	if err == ErrInvalidPath {
		httputil.JSONError(w, "invalid path", http.StatusBadRequest)
		return
	}
	if err != nil {
		httputil.JSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) IncrementView(w http.ResponseWriter, r *http.Request) {
	pagePath := r.PathValue("path")

	count, err := h.svc.IncrementViewCount(pagePath)
	if err == ErrNotFound {
		httputil.JSONError(w, "page not found", http.StatusNotFound)
		return
	}
	if err != nil {
		httputil.JSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]int{"view_count": count})
}

func (h *Handler) GetRSS(w http.ResponseWriter, r *http.Request) {
	feed, err := BuildRSSFeed(h.svc.db, h.baseURL)
	if err != nil {
		http.Error(w, "failed to build feed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/rss+xml; charset=utf-8")
	w.Write(feed)
}

func (h *Handler) GetSitemap(w http.ResponseWriter, r *http.Request) {
	xml, err := BuildSitemap(h.svc.db, h.baseURL)
	if err != nil {
		http.Error(w, "failed to build sitemap", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Write(xml)
}

func extractPath(r *http.Request) string {
	return r.PathValue("path")
}
