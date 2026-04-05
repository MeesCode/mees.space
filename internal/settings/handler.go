package settings

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"mees.space/internal/httputil"
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
	settings := map[string]*string{
		"ai_system_prompt": &resp.AISystemPrompt,
		"ai_api_key":       &resp.AIAPIKey,
		"ai_model":         &resp.AIModel,
	}

	rows, err := h.db.Query("SELECT key, value FROM settings WHERE key IN ('ai_system_prompt', 'ai_api_key', 'ai_model')")
	if err != nil {
		httputil.JSONError(w, "failed to load settings", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			continue
		}
		if dest, ok := settings[key]; ok {
			*dest = value
		}
	}

	if resp.AIModel == "" {
		resp.AIModel = "claude-sonnet-4-6-20250627"
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
		httputil.JSONError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	upsert := func(key, value string) error {
		_, err := h.db.Exec(
			"INSERT INTO settings (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = ?",
			key, value, value,
		)
		return err
	}

	if req.AISystemPrompt != nil {
		if err := upsert("ai_system_prompt", *req.AISystemPrompt); err != nil {
			httputil.JSONError(w, "failed to update settings", http.StatusInternalServerError)
			return
		}
	}

	if req.AIAPIKey != nil {
		if err := upsert("ai_api_key", *req.AIAPIKey); err != nil {
			httputil.JSONError(w, "failed to update settings", http.StatusInternalServerError)
			return
		}
	}

	if req.AIModel != nil {
		if err := upsert("ai_model", *req.AIModel); err != nil {
			httputil.JSONError(w, "failed to update settings", http.StatusInternalServerError)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"ok":true}`))
}
