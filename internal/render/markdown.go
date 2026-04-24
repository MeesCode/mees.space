// Package render converts markdown source into HTML with stable heading
// IDs and hover-reveal anchor links, for server-side rendering of content
// pages. Output matches the client react-markdown + rehype-slug +
// rehype-autolink-headings pipeline closely enough that heading fragment
// links work across both rendering paths.
package render

import (
	"bytes"
	"fmt"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	grenderer "github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// Renderer is concurrency-safe; construct once per process.
type Renderer struct {
	md goldmark.Markdown
}

// New returns a configured Renderer (GFM + raw HTML + heading anchors).
func New() *Renderer {
	md := goldmark.New(
		goldmark.WithExtensions(extension.GFM),
		goldmark.WithParserOptions(
			parser.WithASTTransformers(
				util.Prioritized(&headingIDTransformer{}, 500),
			),
		),
		goldmark.WithRendererOptions(
			html.WithUnsafe(),
			grenderer.WithNodeRenderers(
				util.Prioritized(&headingAnchorRenderer{}, 1),
			),
		),
	)
	return &Renderer{md: md}
}

// ToHTML renders markdown source to HTML bytes.
func (r *Renderer) ToHTML(src []byte) ([]byte, error) {
	var buf bytes.Buffer
	if err := r.md.Convert(src, &buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// headingIDTransformer walks every Heading node and assigns a stable id
// attribute via Slugify.
type headingIDTransformer struct{}

func (t *headingIDTransformer) Transform(doc *ast.Document, reader text.Reader, _ parser.Context) {
	seen := map[string]int{}
	src := reader.Source()

	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		h, ok := n.(*ast.Heading)
		if !ok {
			return ast.WalkContinue, nil
		}
		slug := Slugify(headingText(h, src), seen)
		h.SetAttributeString("id", []byte(slug))
		return ast.WalkContinue, nil
	})
}

// headingAnchorRenderer overrides goldmark's default heading renderer to
// inject a hover-reveal anchor link immediately after the opening tag.
type headingAnchorRenderer struct{}

func (r *headingAnchorRenderer) RegisterFuncs(reg grenderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindHeading, r.renderHeading)
}

func (r *headingAnchorRenderer) renderHeading(
	w util.BufWriter, _ []byte, node ast.Node, entering bool,
) (ast.WalkStatus, error) {
	n := node.(*ast.Heading)
	if entering {
		_, _ = fmt.Fprintf(w, "<h%d", n.Level)

		// emit id attribute
		if idVal, ok := n.AttributeString("id"); ok {
			_, _ = fmt.Fprintf(w, ` id="%s"`, idVal)
		}
		_ = w.WriteByte('>')

		// inject the anchor link with all required attributes
		if idVal, ok := n.AttributeString("id"); ok {
			slug := string(idVal.([]byte))
			_, _ = fmt.Fprintf(w,
				`<a href="#%s" class="heading-anchor" aria-hidden="true" tabindex="-1">#</a>`,
				slug,
			)
		}
	} else {
		_, _ = fmt.Fprintf(w, "</h%d>\n", n.Level)
	}
	return ast.WalkContinue, nil
}

// headingText flattens a heading's inline children to plain text for slugging.
func headingText(h *ast.Heading, source []byte) string {
	var buf bytes.Buffer
	for c := h.FirstChild(); c != nil; c = c.NextSibling() {
		collectText(c, source, &buf)
	}
	return buf.String()
}

func collectText(n ast.Node, source []byte, buf *bytes.Buffer) {
	switch v := n.(type) {
	case *ast.Text:
		buf.Write(v.Segment.Value(source))
	case *ast.String:
		buf.Write(v.Value)
	default:
		for c := n.FirstChild(); c != nil; c = c.NextSibling() {
			collectText(c, source, buf)
		}
	}
}
