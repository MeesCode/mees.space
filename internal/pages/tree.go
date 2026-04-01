package pages

import (
	"database/sql"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type pageInfo struct {
	Title     string
	ShowDate  bool
	CreatedAt string
	Published bool
}

func BuildContentTree(db *sql.DB, contentDir string, includeDrafts bool) ([]TreeNode, error) {
	pages := make(map[string]pageInfo)
	rows, err := db.Query("SELECT path, title, show_date, created_at, published FROM pages")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var path, title, createdAt string
		var showDate, published bool
		if err := rows.Scan(&path, &title, &showDate, &createdAt, &published); err != nil {
			return nil, err
		}
		pages[path] = pageInfo{Title: title, ShowDate: showDate, CreatedAt: createdAt, Published: published}
	}

	return buildTreeRecursive(contentDir, "", pages, includeDrafts), nil
}

func buildTreeRecursive(baseDir, relDir string, pages map[string]pageInfo, includeDrafts bool) []TreeNode {
	var nodes []TreeNode

	dir := baseDir
	if relDir != "" {
		dir = filepath.Join(baseDir, relDir)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nodes
	}

	for _, e := range entries {
		var childRel string
		if relDir == "" {
			childRel = e.Name()
		} else {
			childRel = relDir + "/" + e.Name()
		}

		if e.IsDir() {
			children := buildTreeRecursive(baseDir, childRel, pages, includeDrafts)
			nodes = append(nodes, TreeNode{
				Name:     e.Name(),
				Path:     childRel,
				IsDir:    true,
				Children: children,
			})
		} else if strings.HasSuffix(e.Name(), ".md") {
			pagePath := strings.TrimSuffix(childRel, ".md")
			name := strings.TrimSuffix(e.Name(), ".md")
			info := pages[pagePath]

			if !includeDrafts && !info.Published {
				continue
			}

			title := info.Title
			if title == "" {
				title = name
			}

			nodes = append(nodes, TreeNode{
				Name:      name,
				Path:      pagePath,
				Title:     title,
				IsDir:     false,
				ShowDate:  info.ShowDate,
				CreatedAt: info.CreatedAt,
				Published: info.Published,
			})
		}
	}

	sortNodes(nodes)
	return nodes
}

func sortNodes(nodes []TreeNode) {
	sort.Slice(nodes, func(i, j int) bool {
		// Files before directories
		if nodes[i].IsDir != nodes[j].IsDir {
			return !nodes[i].IsDir
		}
		// Directories: alphabetical ascending
		if nodes[i].IsDir {
			return strings.ToLower(nodes[i].Name) < strings.ToLower(nodes[j].Name)
		}
		// Files: newest first (by created_at descending)
		if nodes[i].CreatedAt != nodes[j].CreatedAt {
			return nodes[i].CreatedAt > nodes[j].CreatedAt
		}
		return strings.ToLower(nodes[i].Name) < strings.ToLower(nodes[j].Name)
	})
}
