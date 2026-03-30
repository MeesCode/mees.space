package pages

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func setupTestDB(t *testing.T) (*sql.DB, string) {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}

	db.Exec(`CREATE TABLE pages (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		path TEXT NOT NULL UNIQUE,
		title TEXT NOT NULL,
		view_count INTEGER NOT NULL DEFAULT 0,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`)
	db.Exec(`CREATE INDEX idx_pages_path ON pages(path)`)

	contentDir := filepath.Join(tmpDir, "content")
	os.MkdirAll(contentDir, 0755)

	t.Cleanup(func() { db.Close() })
	return db, contentDir
}

func TestCreateAndGetPage(t *testing.T) {
	db, contentDir := setupTestDB(t)
	svc := NewService(db, contentDir)

	err := svc.CreatePage("home", "Home Page", "# Welcome\n\nHello world!")
	if err != nil {
		t.Fatalf("CreatePage: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(contentDir, "home.md"))
	if err != nil {
		t.Fatal("file not created")
	}
	if string(content) != "# Welcome\n\nHello world!" {
		t.Errorf("unexpected content: %s", content)
	}

	page, err := svc.GetPage("home")
	if err != nil {
		t.Fatalf("GetPage: %v", err)
	}

	if page.Path != "home" {
		t.Errorf("expected path 'home', got '%s'", page.Path)
	}
	if page.Title != "Home Page" {
		t.Errorf("expected title 'Home Page', got '%s'", page.Title)
	}
	if page.Content != "# Welcome\n\nHello world!" {
		t.Errorf("unexpected content: %s", page.Content)
	}
	if page.ViewCount != 0 {
		t.Errorf("expected view_count 0, got %d", page.ViewCount)
	}
}

func TestCreatePage_NestedPath(t *testing.T) {
	db, contentDir := setupTestDB(t)
	svc := NewService(db, contentDir)

	err := svc.CreatePage("blogs/my-first-post", "My First Post", "Content here")
	if err != nil {
		t.Fatalf("CreatePage nested: %v", err)
	}

	if _, err := os.Stat(filepath.Join(contentDir, "blogs", "my-first-post.md")); err != nil {
		t.Fatal("nested file not created")
	}
}

func TestCreatePage_AlreadyExists(t *testing.T) {
	db, contentDir := setupTestDB(t)
	svc := NewService(db, contentDir)

	svc.CreatePage("home", "Home", "Content")
	err := svc.CreatePage("home", "Home Again", "More content")
	if err != ErrExists {
		t.Errorf("expected ErrExists, got %v", err)
	}
}

func TestUpdatePage(t *testing.T) {
	db, contentDir := setupTestDB(t)
	svc := NewService(db, contentDir)

	svc.CreatePage("home", "Home", "Original")

	err := svc.UpdatePage("home", "Updated Home", "New content")
	if err != nil {
		t.Fatalf("UpdatePage: %v", err)
	}

	page, _ := svc.GetPage("home")
	if page.Title != "Updated Home" {
		t.Errorf("expected title 'Updated Home', got '%s'", page.Title)
	}
	if page.Content != "New content" {
		t.Errorf("expected content 'New content', got '%s'", page.Content)
	}
}

func TestUpdatePage_NotFound(t *testing.T) {
	db, contentDir := setupTestDB(t)
	svc := NewService(db, contentDir)

	err := svc.UpdatePage("nonexistent", "Title", "Content")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestDeletePage(t *testing.T) {
	db, contentDir := setupTestDB(t)
	svc := NewService(db, contentDir)

	svc.CreatePage("home", "Home", "Content")

	err := svc.DeletePage("home")
	if err != nil {
		t.Fatalf("DeletePage: %v", err)
	}

	if _, err := os.Stat(filepath.Join(contentDir, "home.md")); !os.IsNotExist(err) {
		t.Error("file should be deleted")
	}

	_, err = svc.GetPage("home")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestIncrementViewCount(t *testing.T) {
	db, contentDir := setupTestDB(t)
	svc := NewService(db, contentDir)

	svc.CreatePage("home", "Home", "Content")

	count, err := svc.IncrementViewCount("home")
	if err != nil {
		t.Fatalf("IncrementViewCount: %v", err)
	}
	if count != 1 {
		t.Errorf("expected count 1, got %d", count)
	}

	count, err = svc.IncrementViewCount("home")
	if err != nil {
		t.Fatalf("IncrementViewCount (2nd): %v", err)
	}
	if count != 2 {
		t.Errorf("expected count 2, got %d", count)
	}
}

func TestGetPage_SelfHeal(t *testing.T) {
	db, contentDir := setupTestDB(t)
	svc := NewService(db, contentDir)

	// Write file directly without DB row
	os.WriteFile(filepath.Join(contentDir, "orphan.md"), []byte("Orphan content"), 0644)

	page, err := svc.GetPage("orphan")
	if err != nil {
		t.Fatalf("GetPage self-heal: %v", err)
	}

	if page.Title != "orphan" {
		t.Errorf("expected title 'orphan', got '%s'", page.Title)
	}
	if page.Content != "Orphan content" {
		t.Errorf("unexpected content: %s", page.Content)
	}
}

func TestPathTraversal(t *testing.T) {
	db, contentDir := setupTestDB(t)
	svc := NewService(db, contentDir)

	tests := []string{
		"../etc/passwd",
		"../../secret",
		"/absolute/path",
		"",
		".",
	}

	for _, path := range tests {
		err := svc.CreatePage(path, "Test", "Content")
		if err != ErrInvalidPath {
			t.Errorf("path %q: expected ErrInvalidPath, got %v", path, err)
		}
	}
}

func TestBuildContentTree(t *testing.T) {
	db, contentDir := setupTestDB(t)

	os.MkdirAll(filepath.Join(contentDir, "blogs"), 0755)
	os.WriteFile(filepath.Join(contentDir, "home.md"), []byte("Home"), 0644)
	os.WriteFile(filepath.Join(contentDir, "blogs", "post1.md"), []byte("Post 1"), 0644)
	os.WriteFile(filepath.Join(contentDir, "blogs", "post2.md"), []byte("Post 2"), 0644)

	db.Exec("INSERT INTO pages (path, title) VALUES (?, ?)", "home", "Home Page")
	db.Exec("INSERT INTO pages (path, title) VALUES (?, ?)", "blogs/post1", "My First Post")

	tree, err := BuildContentTree(db, contentDir)
	if err != nil {
		t.Fatalf("BuildContentTree: %v", err)
	}

	if len(tree) != 2 {
		t.Fatalf("expected 2 top-level items, got %d", len(tree))
	}

	// Files come first, then dirs
	if tree[0].IsDir {
		t.Error("expected first item to be a file")
	}
	if tree[0].Title != "Home Page" {
		t.Errorf("expected title 'Home Page', got '%s'", tree[0].Title)
	}

	// Blogs directory second
	if !tree[1].IsDir {
		t.Error("expected second item to be a directory")
	}
	if tree[1].Name != "blogs" {
		t.Errorf("expected 'blogs', got '%s'", tree[1].Name)
	}
	if len(tree[1].Children) != 2 {
		t.Errorf("expected 2 children in blogs, got %d", len(tree[1].Children))
	}
}
