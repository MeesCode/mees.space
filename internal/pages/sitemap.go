package pages

import (
	"database/sql"
	"encoding/xml"
	"fmt"
	"time"
)

type sitemapURL struct {
	XMLName xml.Name `xml:"url"`
	Loc     string   `xml:"loc"`
	LastMod string   `xml:"lastmod,omitempty"`
}

type sitemapURLSet struct {
	XMLName xml.Name     `xml:"urlset"`
	XMLNS   string       `xml:"xmlns,attr"`
	URLs    []sitemapURL `xml:"url"`
}

// BuildSitemap returns the full sitemap.xml body for all published pages.
// The `home` path renders as a bare baseURL (no /home suffix).
func BuildSitemap(db *sql.DB, baseURL string) ([]byte, error) {
	rows, err := db.Query(`SELECT path, updated_at FROM pages WHERE published = 1 ORDER BY updated_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("query pages: %w", err)
	}
	defer rows.Close()

	var urls []sitemapURL
	for rows.Next() {
		var path, updatedAt string
		if err := rows.Scan(&path, &updatedAt); err != nil {
			return nil, err
		}
		t, err := time.Parse(time.RFC3339, updatedAt)
		if err != nil {
			t = time.Now()
		}
		loc := baseURL + "/" + path
		if path == "home" {
			loc = baseURL
		}
		urls = append(urls, sitemapURL{
			Loc:     loc,
			LastMod: t.Format("2006-01-02"),
		})
	}

	set := sitemapURLSet{
		XMLNS: "http://www.sitemaps.org/schemas/sitemap/0.9",
		URLs:  urls,
	}

	out, err := xml.MarshalIndent(set, "", "  ")
	if err != nil {
		return nil, err
	}
	return append([]byte(xml.Header), out...), nil
}
