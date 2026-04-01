package settings

import (
	"database/sql"
	"encoding/json"
	"net/http"
)

type Handler struct {
	db *sql.DB
}

func NewHandler(db *sql.DB) *Handler {
	return &Handler{db: db}
}

type SettingsResponse struct {
	AISystemPrompt string `json:"ai_system_prompt"`
	AIAPIKey       string `json:"ai_api_key"`
	AIModel        string `json:"ai_model"`
}

type SettingsRequest struct {
	AISystemPrompt *string `json:"ai_system_prompt,omitempty"`
	AIAPIKey       *string `json:"ai_api_key,omitempty"`
	AIModel        *string `json:"ai_model,omitempty"`
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	resp := SettingsResponse{}

	h.db.QueryRow("SELECT value FROM settings WHERE key = 'ai_system_prompt'").Scan(&resp.AISystemPrompt)
	h.db.QueryRow("SELECT value FROM settings WHERE key = 'ai_api_key'").Scan(&resp.AIAPIKey)
	h.db.QueryRow("SELECT value FROM settings WHERE key = 'ai_model'").Scan(&resp.AIModel)

	if resp.AIModel == "" {
		resp.AIModel = "claude-sonnet-4-20250514"
	}

	// Mask the API key for security — only show last 8 chars
	if len(resp.AIAPIKey) > 8 {
		resp.AIAPIKey = "••••••••" + resp.AIAPIKey[len(resp.AIAPIKey)-8:]
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	var req SettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	if req.AISystemPrompt != nil {
		h.db.Exec("INSERT INTO settings (key, value) VALUES ('ai_system_prompt', ?) ON CONFLICT(key) DO UPDATE SET value = ?", *req.AISystemPrompt, *req.AISystemPrompt)
	}

	if req.AIAPIKey != nil {
		h.db.Exec("INSERT INTO settings (key, value) VALUES ('ai_api_key', ?) ON CONFLICT(key) DO UPDATE SET value = ?", *req.AIAPIKey, *req.AIAPIKey)
	}

	if req.AIModel != nil {
		h.db.Exec("INSERT INTO settings (key, value) VALUES ('ai_model', ?) ON CONFLICT(key) DO UPDATE SET value = ?", *req.AIModel, *req.AIModel)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"ok":true}`))
}
