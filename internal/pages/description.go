package pages

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"
)

var (
	reFencedCode = regexp.MustCompile("(?s)```.*?```")
	reInlineCode = regexp.MustCompile("`[^`]*`")
	reImage      = regexp.MustCompile(`!\[([^\]]*)\]\([^)]*\)`)
	reLink       = regexp.MustCompile(`\[([^\]]+)\]\([^)]*\)`)
	reEmphasis   = regexp.MustCompile(`[*_~]+`)
	reWhitespace = regexp.MustCompile(`\s+`)
)

// contentSnippet strips markdown syntax from content and returns the first
// ≤160 chars at a word boundary. Always safe and fast, never returns error.
func contentSnippet(content string) string {
	s := content

	// Remove fenced code blocks entirely
	s = reFencedCode.ReplaceAllString(s, " ")
	// Replace images with alt text
	s = reImage.ReplaceAllString(s, "$1")
	// Replace links with their text
	s = reLink.ReplaceAllString(s, "$1")
	// Remove inline code backticks (keep contents)
	s = reInlineCode.ReplaceAllStringFunc(s, func(m string) string {
		return strings.Trim(m, "`")
	})
	// Strip emphasis markers
	s = reEmphasis.ReplaceAllString(s, "")

	// Strip leading line markers (#, >, -, *, numbered) at start of each line
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		trimmed := strings.TrimLeft(line, " \t")
		// Headings
		trimmed = strings.TrimLeft(trimmed, "#")
		// Blockquotes
		trimmed = strings.TrimLeft(trimmed, ">")
		// List bullets
		if len(trimmed) > 0 && (trimmed[0] == '-' || trimmed[0] == '*' || trimmed[0] == '+') {
			if len(trimmed) > 1 && trimmed[1] == ' ' {
				trimmed = trimmed[2:]
			}
		}
		// Numbered list (1. / 1) followed by a space — only then treat as list marker
		numIdx := 0
		for numIdx < len(trimmed) && trimmed[numIdx] >= '0' && trimmed[numIdx] <= '9' {
			numIdx++
		}
		if numIdx > 0 && numIdx+1 < len(trimmed) && (trimmed[numIdx] == '.' || trimmed[numIdx] == ')') && trimmed[numIdx+1] == ' ' {
			trimmed = trimmed[numIdx+2:]
		}
		lines[i] = trimmed
	}
	s = strings.Join(lines, " ")

	// Collapse whitespace
	s = reWhitespace.ReplaceAllString(s, " ")
	s = strings.TrimSpace(s)

	// Cap at 160 at word boundary
	if len(s) <= 160 {
		return s
	}
	cut := s[:160]
	if idx := strings.LastIndex(cut, " "); idx > 0 {
		return cut[:idx]
	}
	return cut
}

const descriptionModel = "claude-haiku-4-5-20251001"

const descriptionSystemPrompt = "Write a meta description for a webpage. Output a single sentence, 130-160 characters, no quotes, no trailing punctuation other than a period. Describe what the reader will learn or get from the page, not meta-commentary about the page itself."

// ClaudeRequest is the minimal subset of Anthropic's /v1/messages body we use.
type ClaudeRequest struct {
	Model       string      `json:"model"`
	MaxTokens   int         `json:"max_tokens"`
	System      string      `json:"system,omitempty"`
	Messages    []ClaudeMsg `json:"messages"`
	Temperature float64     `json:"temperature,omitempty"`
}

type ClaudeMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ClaudeClient sends a message to the Anthropic API and returns the response text.
type ClaudeClient interface {
	CreateMessage(ctx context.Context, apiKey string, req ClaudeRequest) (string, error)
}

// httpClaudeClient is the production implementation.
type httpClaudeClient struct {
	http *http.Client
}

func (c *httpClaudeClient) CreateMessage(ctx context.Context, apiKey string, req ClaudeRequest) (string, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return "", err
	}
	hr, err := http.NewRequestWithContext(ctx, "POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	hr.Header.Set("Content-Type", "application/json")
	hr.Header.Set("x-api-key", apiKey)
	hr.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.http.Do(hr)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("anthropic API %d: %s", resp.StatusCode, string(data))
	}
	var parsed struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return "", err
	}
	for _, c := range parsed.Content {
		if c.Type == "text" && c.Text != "" {
			return c.Text, nil
		}
	}
	return "", fmt.Errorf("no text content in response")
}

// Generator produces meta descriptions via Claude, with a content-derived fallback.
type Generator struct {
	db      *sql.DB
	client  ClaudeClient
	timeout time.Duration
}

// NewGenerator returns a Generator that uses the default HTTP Claude client.
func NewGenerator(db *sql.DB, timeout time.Duration) *Generator {
	return &Generator{
		db:      db,
		client:  &httpClaudeClient{http: &http.Client{Timeout: timeout}},
		timeout: timeout,
	}
}

// Generate returns a description for the given page content. Always non-empty:
// AI result, or the content-derived fallback on any failure.
func (g *Generator) Generate(ctx context.Context, title, content string) string {
	apiKey := g.loadAPIKey()
	if apiKey == "" {
		return contentSnippet(content)
	}

	userMsg := "Title: " + title + "\n\nContent:\n" + truncate(content, 4000)
	req := ClaudeRequest{
		Model:       descriptionModel,
		MaxTokens:   120,
		Temperature: 0.3,
		System:      descriptionSystemPrompt,
		Messages:    []ClaudeMsg{{Role: "user", Content: userMsg}},
	}

	callCtx, cancel := context.WithTimeout(ctx, g.timeout)
	defer cancel()

	text, err := g.client.CreateMessage(callCtx, apiKey, req)
	if err != nil {
		log.Printf("description: AI generation failed: %v", err)
		return contentSnippet(content)
	}
	return postprocess(text, content)
}

func (g *Generator) loadAPIKey() string {
	var key string
	row := g.db.QueryRow(`SELECT value FROM settings WHERE key = 'ai_api_key'`)
	if err := row.Scan(&key); err != nil {
		return ""
	}
	return key
}

func postprocess(text, content string) string {
	s := strings.TrimSpace(text)
	// Strip surrounding quotes (ASCII + curly)
	for _, q := range []string{`"`, `'`, "“", "”", "‘", "’"} {
		s = strings.TrimPrefix(s, q)
		s = strings.TrimSuffix(s, q)
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return contentSnippet(content)
	}
	// Cap at 160 at word boundary
	if len(s) <= 160 {
		return s
	}
	cut := s[:160]
	if idx := strings.LastIndex(cut, " "); idx > 0 {
		return cut[:idx]
	}
	return cut
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}

// BackfillEmpty fills empty descriptions for published pages one at a time.
// Drafts (published = 0) are skipped to avoid spending API tokens on
// unpublished content. Intended to run once on server startup in a background
// goroutine. Returns when no more empty descriptions exist or ctx is canceled.
func (g *Generator) BackfillEmpty(ctx context.Context, svc *Service) {
	const delay = 500 * time.Millisecond

	for {
		if ctx.Err() != nil {
			return
		}

		var path string
		err := g.db.QueryRowContext(ctx, `
			SELECT path FROM pages
			WHERE description = '' AND published = 1
			ORDER BY updated_at DESC
			LIMIT 1
		`).Scan(&path)
		if err == sql.ErrNoRows {
			return
		}
		if err != nil {
			log.Printf("backfill: query failed: %v", err)
			return
		}

		page, err := svc.GetPage(path)
		if err != nil {
			log.Printf("backfill: load %s failed: %v", path, err)
			// Mark with a space so we don't retry forever on a single broken page.
			if _, err := g.db.ExecContext(ctx, `UPDATE pages SET description = ' ' WHERE path = ?`, path); err != nil {
				log.Printf("backfill: mark-broken write failed for %s: %v", path, err)
				return
			}
			continue
		}

		desc := g.Generate(ctx, page.Title, page.Content)
		if _, err := g.db.ExecContext(ctx, `UPDATE pages SET description = ? WHERE path = ? AND description = ''`, desc, path); err != nil {
			log.Printf("backfill: write %s failed: %v", path, err)
			return
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}
	}
}
