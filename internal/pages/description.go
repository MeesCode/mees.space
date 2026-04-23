package pages

import (
	"regexp"
	"strings"
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
		// Numbered list (1. etc.)
		for len(trimmed) > 0 && trimmed[0] >= '0' && trimmed[0] <= '9' {
			trimmed = trimmed[1:]
		}
		trimmed = strings.TrimLeft(trimmed, ".)")
		trimmed = strings.TrimLeft(trimmed, " ")
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
