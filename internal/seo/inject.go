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

// Injector holds the index.html template and produces customized HTML.
type Injector struct {
	template []byte
}

// NewInjector reads dist/index.html and verifies the </head> anchor exists.
// Returns an error if the file is missing or the anchor can't be found.
func NewInjector(distDir string) (*Injector, error) {
	path := filepath.Join(distDir, "index.html")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	if !bytes.Contains(data, []byte("</head>")) {
		return nil, fmt.Errorf("%s: missing </head> anchor; cannot inject SEO metadata", path)
	}
	return &Injector{template: data}, nil
}

// Inject returns a customized copy of the template with per-page <head> tags
// inserted just before the first </head>. Safe for concurrent use.
func (i *Injector) Inject(m PageMeta) []byte {
	fragment := buildFragment(m)
	replacement := append(fragment, []byte("</head>")...)
	return bytes.Replace(i.template, []byte("</head>"), replacement, 1)
}

// Raw returns the unmodified template.
func (i *Injector) Raw() []byte {
	return i.template
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
