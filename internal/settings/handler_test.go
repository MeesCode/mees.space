package settings

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	_, err = db.Exec(`CREATE TABLE settings (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL DEFAULT ''
	)`)
	if err != nil {
		t.Fatalf("create table: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestGetEmpty(t *testing.T) {
	db := setupTestDB(t)
	h := NewHandler(db)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/settings", nil)
	h.Get(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	var resp SettingsResponse
	json.NewDecoder(rr.Body).Decode(&resp)

	if resp.AIModel != "claude-sonnet-4-6" {
		t.Errorf("default model = %q, want claude-sonnet-4-6", resp.AIModel)
	}
	if resp.AIAPIKey != "" {
		t.Errorf("api key should be empty, got %q", resp.AIAPIKey)
	}
}

func TestUpdateAndGet(t *testing.T) {
	db := setupTestDB(t)
	h := NewHandler(db)

	// Update settings
	prompt := "You are a helpful assistant"
	descPrompt := "Write a 100-char tagline."
	key := "sk-test-1234567890abcdef"
	model := "claude-haiku-4-5"
	body, _ := json.Marshal(SettingsRequest{
		AISystemPrompt:      &prompt,
		AIDescriptionPrompt: &descPrompt,
		AIAPIKey:            &key,
		AIModel:             &model,
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest("PUT", "/api/settings", bytes.NewReader(body))
	h.Update(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("update status = %d, want %d", rr.Code, http.StatusOK)
	}

	// Get settings back
	rr = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/api/settings", nil)
	h.Get(rr, req)

	var resp SettingsResponse
	json.NewDecoder(rr.Body).Decode(&resp)

	if resp.AISystemPrompt != prompt {
		t.Errorf("system prompt = %q, want %q", resp.AISystemPrompt, prompt)
	}
	if resp.AIDescriptionPrompt != descPrompt {
		t.Errorf("description prompt = %q, want %q", resp.AIDescriptionPrompt, descPrompt)
	}
	if resp.AIModel != model {
		t.Errorf("model = %q, want %q", resp.AIModel, model)
	}
	// API key should be masked
	if resp.AIAPIKey == key {
		t.Error("API key should be masked, got raw value")
	}
	if len(resp.AIAPIKey) == 0 {
		t.Error("masked API key should not be empty")
	}
}

func TestUpdatePartial(t *testing.T) {
	db := setupTestDB(t)
	h := NewHandler(db)

	// Only update model
	model := "claude-opus-4-7"
	body, _ := json.Marshal(SettingsRequest{
		AIModel: &model,
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest("PUT", "/api/settings", bytes.NewReader(body))
	h.Update(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	// Verify only model was set
	rr = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/api/settings", nil)
	h.Get(rr, req)

	var resp SettingsResponse
	json.NewDecoder(rr.Body).Decode(&resp)

	if resp.AIModel != model {
		t.Errorf("model = %q, want %q", resp.AIModel, model)
	}
	if resp.AISystemPrompt != "" {
		t.Errorf("system prompt should be empty, got %q", resp.AISystemPrompt)
	}
}

func TestUpdateInvalidBody(t *testing.T) {
	db := setupTestDB(t)
	h := NewHandler(db)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest("PUT", "/api/settings", bytes.NewReader([]byte("invalid json")))
	h.Update(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestGetIncludesDescriptionPromptDefault(t *testing.T) {
	db := setupTestDB(t)
	h := NewHandler(db)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/settings", nil)
	h.Get(rr, req)

	var resp SettingsResponse
	json.NewDecoder(rr.Body).Decode(&resp)

	if resp.AIDescriptionPromptDefault == "" {
		t.Error("AIDescriptionPromptDefault should always be populated, got empty string")
	}
	if !strings.Contains(resp.AIDescriptionPromptDefault, "meta description") {
		t.Errorf("AIDescriptionPromptDefault = %q, expected to reference meta description", resp.AIDescriptionPromptDefault)
	}
}

func TestAPIKeyMasking(t *testing.T) {
	db := setupTestDB(t)
	h := NewHandler(db)

	// Set a short key (<=8 chars) - should not be masked
	shortKey := "short"
	body, _ := json.Marshal(SettingsRequest{AIAPIKey: &shortKey})
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("PUT", "/api/settings", bytes.NewReader(body))
	h.Update(rr, req)

	rr = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/api/settings", nil)
	h.Get(rr, req)

	var resp SettingsResponse
	json.NewDecoder(rr.Body).Decode(&resp)

	// Short key returned as-is
	if resp.AIAPIKey != shortKey {
		t.Errorf("short key = %q, want %q", resp.AIAPIKey, shortKey)
	}

	// Set a long key
	longKey := "sk-ant-1234567890abcdef"
	body, _ = json.Marshal(SettingsRequest{AIAPIKey: &longKey})
	rr = httptest.NewRecorder()
	req = httptest.NewRequest("PUT", "/api/settings", bytes.NewReader(body))
	h.Update(rr, req)

	rr = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/api/settings", nil)
	h.Get(rr, req)

	json.NewDecoder(rr.Body).Decode(&resp)

	if resp.AIAPIKey == longKey {
		t.Error("long key should be masked")
	}
	// Masked key should end with last 8 chars of original
	suffix := longKey[len(longKey)-8:]
	if !strings.HasSuffix(resp.AIAPIKey, suffix) {
		t.Errorf("masked key should end with %q, got %q", suffix, resp.AIAPIKey)
	}
	if !strings.HasPrefix(resp.AIAPIKey, "••••••••") {
		t.Errorf("masked key should start with dots, got %q", resp.AIAPIKey)
	}
}
