package pages

import (
	"database/sql"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func BuildContentTree(db *sql.DB, contentDir string) ([]TreeNode, error) {
	titles := make(map[string]string)
	rows, err := db.Query("SELECT path, title FROM pages")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var path, title string
		if err := rows.Scan(&path, &title); err != nil {
			return nil, err
		}
		titles[path] = title
	}

	return buildTreeRecursive(contentDir, "", titles), nil
}

func buildTreeRecursive(baseDir, relDir string, titles map[string]string) []TreeNode {
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
			children := buildTreeRecursive(baseDir, childRel, titles)
			nodes = append(nodes, TreeNode{
				Name:     e.Name(),
				Path:     childRel,
				IsDir:    true,
				Children: children,
			})
		} else if strings.HasSuffix(e.Name(), ".md") {
			pagePath := strings.TrimSuffix(childRel, ".md")
			name := strings.TrimSuffix(e.Name(), ".md")
			title := titles[pagePath]
			if title == "" {
				title = name
			}

			nodes = append(nodes, TreeNode{
				Name:  name,
				Path:  pagePath,
				Title: title,
				IsDir: false,
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
