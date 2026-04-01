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
}

func BuildContentTree(db *sql.DB, contentDir string) ([]TreeNode, error) {
	pages := make(map[string]pageInfo)
	rows, err := db.Query("SELECT path, title, show_date, created_at FROM pages")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var path, title, createdAt string
		var showDate bool
		if err := rows.Scan(&path, &title, &showDate, &createdAt); err != nil {
			return nil, err
		}
		pages[path] = pageInfo{Title: title, ShowDate: showDate, CreatedAt: createdAt}
	}

	return buildTreeRecursive(contentDir, "", pages), nil
}

func buildTreeRecursive(baseDir, relDir string, pages map[string]pageInfo) []TreeNode {
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
			children := buildTreeRecursive(baseDir, childRel, pages)
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
		return strings.ToLower(nodes[i].Name) < strings.ToLower(nodes[j].Name)
	})
}
