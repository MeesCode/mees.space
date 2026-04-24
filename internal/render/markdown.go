// Package render converts markdown source into HTML with stable heading
// IDs and hover-reveal anchor links, for server-side rendering of content
// pages. It is designed to produce output compatible with the client's
// react-markdown + rehype-slug + rehype-autolink-headings pipeline so that
// heading fragment links work across both rendering paths.
package render

import (
	"bytes"
	"fmt"
)

// Renderer holds a configured goldmark pipeline and is safe for concurrent use.
type Renderer struct{}

// New returns a Renderer with GFM, raw HTML pass-through, and the heading
// anchor transform enabled.
func New() *Renderer {
	return &Renderer{}
}

// ToHTML renders markdown source to HTML bytes.
// The empty implementation is fleshed out in Task 8.
func (r *Renderer) ToHTML(src []byte) ([]byte, error) {
	return nil, fmt.Errorf("not implemented")
}

var _ = bytes.MinRead // keep import during stub stage
