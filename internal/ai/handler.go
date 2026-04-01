package ai

import (
	"bufio"
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type Handler struct {
	db *sql.DB
}

func NewHandler(db *sql.DB) *Handler {
	return &Handler{db: db}
}

type PromptRequest struct {
	Prompt  string `json:"prompt"`
	Content string `json:"content"`
}

// Anthropic API types
type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	Stream    bool               `json:"stream"`
	System    string             `json:"system,omitempty"`
	Messages  []anthropicMessage `json:"messages"`
}

func (h *Handler) Complete(w http.ResponseWriter, r *http.Request) {
	var req PromptRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeSSEError(w, "invalid request body")
		return
	}

	if req.Prompt == "" {
		writeSSEError(w, "prompt is required")
		return
	}

	// Get API key, system prompt, and model from settings
	var apiKey, systemPrompt, model string
	h.db.QueryRow("SELECT value FROM settings WHERE key = 'ai_api_key'").Scan(&apiKey)
	h.db.QueryRow("SELECT value FROM settings WHERE key = 'ai_system_prompt'").Scan(&systemPrompt)
	h.db.QueryRow("SELECT value FROM settings WHERE key = 'ai_model'").Scan(&model)
	if model == "" {
		model = "claude-sonnet-4-20250514"
	}

	if apiKey == "" {
		writeSSEError(w, "AI API key not configured. Go to Settings to add it.")
		return
	}

	// Build the user message
	userMessage := req.Prompt
	if req.Content != "" {
		userMessage = fmt.Sprintf("Here is the current markdown content:\n\n---\n%s\n---\n\nInstruction: %s", req.Content, req.Prompt)
	}

	// Call Anthropic API with streaming
	apiReq := anthropicRequest{
		Model:     model,
		MaxTokens: 4096,
		Stream:    true,
		System:    systemPrompt,
		Messages: []anthropicMessage{
			{Role: "user", Content: userMessage},
		},
	}

	body, err := json.Marshal(apiReq)
	if err != nil {
		writeSSEError(w, "failed to build request")
		return
	}

	httpReq, err := http.NewRequest("POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(body))
	if err != nil {
		writeSSEError(w, "failed to create request")
		return
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	client := &http.Client{Timeout: 300 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		writeSSEError(w, fmt.Sprintf("failed to call AI API: %v", err))
		return
	}
	defer resp.Body.Close()

	// If Anthropic returned an error status, read full body and report
	if resp.StatusCode != http.StatusOK {
		var buf bytes.Buffer
		buf.ReadFrom(resp.Body)
		var errResp struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if json.Unmarshal(buf.Bytes(), &errResp) == nil && errResp.Error.Message != "" {
			writeSSEError(w, errResp.Error.Message)
		} else {
			writeSSEError(w, fmt.Sprintf("AI API returned %d", resp.StatusCode))
		}
		return
	}

	// Set up SSE response
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeSSEError(w, "streaming not supported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Parse Anthropic SSE stream
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 64*1024), 512*1024)

	for scanner.Scan() {
		line := scanner.Text()

		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")

		if data == "[DONE]" {
			break
		}

		var event struct {
			Type  string `json:"type"`
			Delta *struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"delta,omitempty"`
		}

		if err := json.Unmarshal([]byte(data), &event); err != nil {
			continue
		}

		if event.Type == "content_block_delta" && event.Delta != nil && event.Delta.Type == "text_delta" {
			sseData, _ := json.Marshal(map[string]string{"text": event.Delta.Text})
			fmt.Fprintf(w, "data: %s\n\n", sseData)
			flusher.Flush()
		}

		if event.Type == "message_stop" {
			break
		}
	}

	fmt.Fprintf(w, "data: [DONE]\n\n")
	flusher.Flush()
}

func writeSSEError(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	data, _ := json.Marshal(map[string]string{"error": msg})
	fmt.Fprintf(w, "data: %s\n\n", data)
	fmt.Fprintf(w, "data: [DONE]\n\n")
}
