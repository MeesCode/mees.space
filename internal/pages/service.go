package pages

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var (
	ErrNotFound    = errors.New("page not found")
	ErrInvalidPath = errors.New("invalid path")
	ErrExists      = errors.New("page already exists")
)

type Service struct {
	db         *sql.DB
	contentDir string
}

func NewService(db *sql.DB, contentDir string) *Service {
	return &Service{db: db, contentDir: contentDir}
}

func (s *Service) GetPage(pagePath string) (*PageResponse, error) {
	clean, err := sanitizePath(pagePath)
	if err != nil {
		return nil, ErrInvalidPath
	}

	filePath := filepath.Join(s.contentDir, clean+".md")
	if !s.isWithinContentDir(filePath) {
		return nil, ErrInvalidPath
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("read file: %w", err)
	}

	var resp PageResponse
	err = s.db.QueryRow(
		"SELECT path, title, view_count, created_at, updated_at, show_date, published FROM pages WHERE path = ?",
		clean,
	).Scan(&resp.Path, &resp.Title, &resp.ViewCount, &resp.CreatedAt, &resp.UpdatedAt, &resp.ShowDate, &resp.Published)
	if err == sql.ErrNoRows {
		// Self-heal: file exists but DB row missing
		title := filepath.Base(clean)
		now := time.Now().UTC().Format(time.RFC3339)
		_, err = s.db.Exec(
			"INSERT INTO pages (path, title, created_at, updated_at, published) VALUES (?, ?, ?, ?, 1)",
			clean, title, now, now,
		)
		if err != nil {
			return nil, fmt.Errorf("self-heal insert: %w", err)
		}
		resp = PageResponse{
			Path:      clean,
			Title:     title,
			ViewCount: 0,
			CreatedAt: now,
			UpdatedAt: now,
			Published: true,
		}
	} else if err != nil {
		return nil, fmt.Errorf("query page: %w", err)
	}

	resp.Content = string(content)
	return &resp, nil
}

func (s *Service) CreatePage(pagePath, title, content string) error {
	clean, err := sanitizePath(pagePath)
	if err != nil {
		return ErrInvalidPath
	}

	filePath := filepath.Join(s.contentDir, clean+".md")
	if !s.isWithinContentDir(filePath) {
		return ErrInvalidPath
	}

	if _, err := os.Stat(filePath); err == nil {
		return ErrExists
	}

	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	_, err = s.db.Exec(
		"INSERT INTO pages (path, title, created_at, updated_at) VALUES (?, ?, ?, ?)",
		clean, title, now, now,
	)
	if err != nil {
		os.Remove(filePath)
		return fmt.Errorf("insert page: %w", err)
	}

	return nil
}

func (s *Service) UpdatePage(pagePath, title, content string, showDate *bool, published *bool, createdAt *string) error {
	clean, err := sanitizePath(pagePath)
	if err != nil {
		return ErrInvalidPath
	}

	filePath := filepath.Join(s.contentDir, clean+".md")
	if !s.isWithinContentDir(filePath) {
		return ErrInvalidPath
	}

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return ErrNotFound
	}

	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)

	// Build dynamic update
	setClauses := []string{"title = ?", "updated_at = ?"}
	args := []interface{}{title, now}

	if showDate != nil {
		setClauses = append(setClauses, "show_date = ?")
		args = append(args, *showDate)
	}
	if published != nil {
		setClauses = append(setClauses, "published = ?")
		args = append(args, *published)
	}
	if createdAt != nil {
		setClauses = append(setClauses, "created_at = ?")
		args = append(args, *createdAt)
	}

	args = append(args, clean)
	query := "UPDATE pages SET " + strings.Join(setClauses, ", ") + " WHERE path = ?"

	result, err := s.db.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("update page: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		// DB row missing, insert it
		sd := false
		if showDate != nil {
			sd = *showDate
		}
		pub := true
		if published != nil {
			pub = *published
		}
		_, err = s.db.Exec(
			"INSERT INTO pages (path, title, created_at, updated_at, show_date, published) VALUES (?, ?, ?, ?, ?, ?)",
			clean, title, now, now, sd, pub,
		)
		if err != nil {
			return fmt.Errorf("insert missing page row: %w", err)
		}
	}

	return nil
}

func (s *Service) DeletePage(pagePath string) error {
	clean, err := sanitizePath(pagePath)
	if err != nil {
		return ErrInvalidPath
	}

	filePath := filepath.Join(s.contentDir, clean+".md")
	if !s.isWithinContentDir(filePath) {
		return ErrInvalidPath
	}

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return ErrNotFound
	}

	if err := os.Remove(filePath); err != nil {
		return fmt.Errorf("remove file: %w", err)
	}

	s.db.Exec("DELETE FROM pages WHERE path = ?", clean)
	return nil
}

func (s *Service) IncrementViewCount(pagePath string) (int, error) {
	clean, err := sanitizePath(pagePath)
	if err != nil {
		return 0, ErrInvalidPath
	}

	result, err := s.db.Exec(
		"UPDATE pages SET view_count = view_count + 1 WHERE path = ?",
		clean,
	)
	if err != nil {
		return 0, fmt.Errorf("increment view count: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return 0, ErrNotFound
	}

	var count int
	err = s.db.QueryRow("SELECT view_count FROM pages WHERE path = ?", clean).Scan(&count)
	if err != nil {
		return 0, err
	}

	return count, nil
}

func sanitizePath(raw string) (string, error) {
	if raw == "" {
		return "", ErrInvalidPath
	}

	// Reject paths that start with / or \ (absolute)
	if strings.HasPrefix(raw, "/") || strings.HasPrefix(raw, "\\") {
		return "", ErrInvalidPath
	}

	cleaned := filepath.ToSlash(filepath.Clean(raw))
	cleaned = strings.TrimPrefix(cleaned, "/")

	if cleaned == "" || cleaned == "." {
		return "", ErrInvalidPath
	}

	if filepath.IsAbs(cleaned) || strings.Contains(cleaned, "..") {
		return "", ErrInvalidPath
	}

	for _, r := range cleaned {
		if r == 0 {
			return "", ErrInvalidPath
		}
	}

	return cleaned, nil
}

func (s *Service) isWithinContentDir(path string) bool {
	absContent, err := filepath.Abs(s.contentDir)
	if err != nil {
		return false
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	return strings.HasPrefix(absPath, absContent+string(filepath.Separator)) || strings.HasPrefix(absPath, absContent+"/")
}
