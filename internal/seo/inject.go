package seo

import (
	"bytes"
	"fmt"
	"html"
	"os"
	"path/filepath"
)

// PageMeta is the per-request data needed to customize the HTML head.
type PageMeta struct {
	Title        string
	Description  string
	CanonicalURL string
	OGImage      string
	NoIndex      bool
}

// BodyInjection carries server-rendered content and a JSON bootstrap payload.
// Either field may be nil/empty; in that case the corresponding marker in the
// HTML shell is replaced with the empty string.
type BodyInjection struct {
	HTML []byte // rendered markdown, inserted into the SSR_CONTENT slot
	Data []byte // JSON bytes; wrapped in a <script id="__page_data__"> tag
}

const (
	markerContent = "<!--SSR_CONTENT-->"
	markerData    = "<!--SSR_DATA-->"
)

// Injector holds the index.html template and produces customized HTML.
type Injector struct {
	template []byte
}

// NewInjector reads dist/index.html and verifies required anchors exist.
func NewInjector(distDir string) (*Injector, error) {
	path := filepath.Join(distDir, "index.html")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	for _, anchor := range [...]string{"</head>", markerContent, markerData} {
		if !bytes.Contains(data, []byte(anchor)) {
			return nil, fmt.Errorf("%s: missing %q anchor; cannot inject", path, anchor)
		}
	}
	return &Injector{template: data}, nil
}

// Inject returns a customized copy of the template with per-page <head>
// tags inserted, rendered body HTML substituted for the content marker,
// and a JSON bootstrap script substituted for the data marker. Safe for
// concurrent use.
func (i *Injector) Inject(m PageMeta, body BodyInjection) []byte {
	out := make([]byte, len(i.template))
	copy(out, i.template)

	// Head fragment.
	fragment := buildFragment(m)
	out = bytes.Replace(out, []byte("</head>"), append(fragment, []byte("</head>")...), 1)

	// Content slot.
	out = bytes.Replace(out, []byte(markerContent), body.HTML, 1)

	// Data slot.
	var dataReplacement []byte
	if len(body.Data) > 0 {
		var b bytes.Buffer
		b.WriteString(`<script id="__page_data__" type="application/json">`)
		b.Write(body.Data)
		b.WriteString(`</script>`)
		dataReplacement = b.Bytes()
	}
	out = bytes.Replace(out, []byte(markerData), dataReplacement, 1)

	return out
}

// Raw returns the template with both SSR markers stripped, for use on
// admin routes that skip SEO injection.
func (i *Injector) Raw() []byte {
	out := make([]byte, len(i.template))
	copy(out, i.template)
	out = bytes.Replace(out, []byte(markerContent), nil, 1)
	out = bytes.Replace(out, []byte(markerData), nil, 1)
	return out
}

func buildFragment(m PageMeta) []byte {
	title := html.EscapeString(m.Title)
	desc := html.EscapeString(m.Description)
	canon := html.EscapeString(m.CanonicalURL)
	img := html.EscapeString(m.OGImage)

	var b bytes.Buffer
	fmt.Fprintf(&b, `<title>%s</title>`, title)
	fmt.Fprintf(&b, `<meta name="description" content="%s">`, desc)
	if canon != "" {
		fmt.Fprintf(&b, `<link rel="canonical" href="%s">`, canon)
	}
	fmt.Fprintf(&b, `<meta property="og:title" content="%s">`, title)
	fmt.Fprintf(&b, `<meta property="og:description" content="%s">`, desc)
	if canon != "" {
		fmt.Fprintf(&b, `<meta property="og:url" content="%s">`, canon)
	}
	if img != "" {
		fmt.Fprintf(&b, `<meta property="og:image" content="%s">`, img)
	}
	fmt.Fprintf(&b, `<meta property="og:type" content="article">`)
	fmt.Fprintf(&b, `<meta name="twitter:card" content="summary_large_image">`)
	fmt.Fprintf(&b, `<meta name="twitter:title" content="%s">`, title)
	fmt.Fprintf(&b, `<meta name="twitter:description" content="%s">`, desc)
	if img != "" {
		fmt.Fprintf(&b, `<meta name="twitter:image" content="%s">`, img)
	}
	if m.NoIndex {
		fmt.Fprintf(&b, `<meta name="robots" content="noindex">`)
	}
	return b.Bytes()
}
