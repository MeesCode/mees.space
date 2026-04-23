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

type ChatMessage struct {
	Role        string `json:"role"` // "user" or "assistant"
	Text        string `json:"text"`
	ContentEdit bool   `json:"content_edit,omitempty"`
}

type PromptRequest struct {
	Prompt  string        `json:"prompt"`
	Content string        `json:"content"`
	History []ChatMessage `json:"history"`
}

// Anthropic API types
type anthropicContent struct {
	Type      string `json:"type"`
	Text      string `json:"text,omitempty"`
	ID        string `json:"id,omitempty"`
	Name      string `json:"name,omitempty"`
	Input     any    `json:"input,omitempty"`
	ToolUseID string `json:"tool_use_id,omitempty"`
	Content   string `json:"content,omitempty"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"` // string or []anthropicContent
}

type anthropicTool struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	InputSchema any    `json:"input_schema"`
}

type anthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	Stream    bool               `json:"stream"`
	System    string             `json:"system,omitempty"`
	Messages  []anthropicMessage `json:"messages"`
	Tools     []anthropicTool    `json:"tools,omitempty"`
}

// SSE event types sent to frontend
type sseEvent struct {
	Type    string `json:"type,omitempty"`    // "text" or "content"
	Text    string `json:"text,omitempty"`    // for type=text (chat response)
	Content string `json:"content,omitempty"` // for type=content (markdown artifact)
	Error   string `json:"error,omitempty"`
}

var updateContentTool = anthropicTool{
	Name:        "update_content",
	Description: "Replace the current page content with new markdown. Use this tool when you need to create, edit, or rewrite the markdown content. Your text response (outside this tool) is shown as a chat message to the user — use it to explain what you did or answer questions.",
	InputSchema: json.RawMessage(`{
		"type": "object",
		"properties": {
			"markdown": {
				"type": "string",
				"description": "The complete new markdown content for the page"
			}
		},
		"required": ["markdown"]
	}`),
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
	settings := map[string]*string{
		"ai_api_key":       &apiKey,
		"ai_system_prompt": &systemPrompt,
		"ai_model":         &model,
	}
	rows, err := h.db.Query("SELECT key, value FROM settings WHERE key IN ('ai_api_key', 'ai_system_prompt', 'ai_model')")
	if err != nil {
		writeSSEError(w, "failed to load settings")
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
	if model == "" {
		model = "claude-sonnet-4-6"
	}

	if apiKey == "" {
		writeSSEError(w, "AI API key not configured. Go to Settings to add it.")
		return
	}

	// Build system prompt with tool usage guidance
	fullSystemPrompt := systemPrompt
	if fullSystemPrompt != "" {
		fullSystemPrompt += "\n\n"
	}
	fullSystemPrompt += "You are an AI assistant integrated into a markdown content editor. " +
		"When the user asks you to write, edit, or modify content, use the update_content tool to provide the new markdown. " +
		"Your text responses are shown in a chat panel — use them to explain changes, answer questions, or discuss ideas. " +
		"You don't always need to use the tool; only use it when the user wants content changes. " +
		"IMPORTANT: Always include a short text response alongside any tool use — briefly explain what you changed or did. Never respond with only a tool call and no text.\n\n" +
		"Chat history context: Messages marked with [You edited the page content] indicate where you previously used the update_content tool. " +
		"The user may have reverted your changes — always trust the current content provided with the latest message as the actual state, not your memory of past edits. " +
		"Only the last user message is the active instruction. Earlier messages are context only."

	// Build messages from history + current prompt
	var messages []anthropicMessage
	for _, msg := range req.History {
		text := msg.Text
		if msg.ContentEdit {
			text = "[You edited the page content]"
		}
		if text == "" {
			continue
		}
		messages = append(messages, anthropicMessage{Role: msg.Role, Content: text})
	}

	// Build the current user message
	userMessage := req.Prompt
	if req.Content != "" {
		userMessage = fmt.Sprintf("Here is the current markdown content:\n\n---\n%s\n---\n\nInstruction: %s", req.Content, req.Prompt)
	}
	messages = append(messages, anthropicMessage{Role: "user", Content: userMessage})

	// Call Anthropic API with streaming and tool use
	apiReq := anthropicRequest{
		Model:     model,
		MaxTokens: 4096,
		Stream:    true,
		System:    fullSystemPrompt,
		Messages:  messages,
		Tools:     []anthropicTool{updateContentTool},
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

	// Parse Anthropic SSE stream, tracking content block types
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 64*1024), 512*1024)

	var currentBlockType string // "text" or "tool_use"
	var toolInputJSON string    // accumulated JSON for tool_use input

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
			Type         string `json:"type"`
			Index        int    `json:"index"`
			ContentBlock *struct {
				Type string `json:"type"`
				Name string `json:"name,omitempty"`
			} `json:"content_block,omitempty"`
			Delta *struct {
				Type        string `json:"type"`
				Text        string `json:"text,omitempty"`
				PartialJSON string `json:"partial_json,omitempty"`
			} `json:"delta,omitempty"`
		}

		if err := json.Unmarshal([]byte(data), &event); err != nil {
			continue
		}

		switch event.Type {
		case "content_block_start":
			if event.ContentBlock != nil {
				currentBlockType = event.ContentBlock.Type
				if currentBlockType == "tool_use" {
					toolInputJSON = ""
					// Notify frontend that content generation is starting
					evt := sseEvent{Type: "content_start"}
					sseData, _ := json.Marshal(evt)
					fmt.Fprintf(w, "data: %s\n\n", sseData)
					flusher.Flush()
				}
			}

		case "content_block_delta":
			if event.Delta == nil {
				continue
			}
			if event.Delta.Type == "text_delta" && currentBlockType == "text" {
				// Stream text response as chat
				evt := sseEvent{Type: "text", Text: event.Delta.Text}
				sseData, _ := json.Marshal(evt)
				fmt.Fprintf(w, "data: %s\n\n", sseData)
				flusher.Flush()
			} else if event.Delta.Type == "input_json_delta" && currentBlockType == "tool_use" {
				// Accumulate tool input JSON
				toolInputJSON += event.Delta.PartialJSON
			}

		case "content_block_stop":
			if currentBlockType == "tool_use" && toolInputJSON != "" {
				// Send final complete content
				var toolInput struct {
					Markdown string `json:"markdown"`
				}
				if json.Unmarshal([]byte(toolInputJSON), &toolInput) == nil && toolInput.Markdown != "" {
					evt := sseEvent{Type: "content", Content: toolInput.Markdown}
					sseData, _ := json.Marshal(evt)
					fmt.Fprintf(w, "data: %s\n\n", sseData)
					flusher.Flush()
				}
			}
			currentBlockType = ""

		case "message_stop":
			goto done
		}
	}

done:
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
