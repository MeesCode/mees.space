package pages

import (
	"database/sql"
	"encoding/xml"
	"fmt"
	"time"
)

type rssChannel struct {
	XMLName     xml.Name  `xml:"channel"`
	Title       string    `xml:"title"`
	Link        string    `xml:"link"`
	Description string    `xml:"description"`
	Items       []rssItem `xml:"item"`
}

type rssItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description,omitempty"`
	PubDate     string `xml:"pubDate"`
	GUID        string `xml:"guid"`
}

type rssFeed struct {
	XMLName xml.Name   `xml:"rss"`
	Version string     `xml:"version,attr"`
	Channel rssChannel `xml:"channel"`
}

func BuildRSSFeed(db *sql.DB, baseURL string) ([]byte, error) {
	rows, err := db.Query(
		"SELECT path, title, created_at FROM pages WHERE published = 1 ORDER BY created_at DESC LIMIT 50",
	)
	if err != nil {
		return nil, fmt.Errorf("query pages: %w", err)
	}
	defer rows.Close()

	var items []rssItem
	for rows.Next() {
		var path, title, createdAt string
		if err := rows.Scan(&path, &title, &createdAt); err != nil {
			return nil, err
		}

		// Parse and format date to RFC1123Z for RSS
		t, err := time.Parse(time.RFC3339, createdAt)
		if err != nil {
			t = time.Now()
		}

		items = append(items, rssItem{
			Title:   title,
			Link:    baseURL + "/" + path,
			PubDate: t.Format(time.RFC1123Z),
			GUID:    baseURL + "/" + path,
		})
	}

	feed := rssFeed{
		Version: "2.0",
		Channel: rssChannel{
			Title:       "Mees Brinkhuis",
			Link:        baseURL,
			Description: "System Architect — thoughts, recipes, and projects",
			Items:       items,
		},
	}

	output, err := xml.MarshalIndent(feed, "", "  ")
	if err != nil {
		return nil, err
	}

	return append([]byte(xml.Header), output...), nil
}
