package auth

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"golang.org/x/crypto/bcrypt"
	_ "modernc.org/sqlite"
)

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}

	db.Exec(`CREATE TABLE users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT NOT NULL UNIQUE,
		password_hash TEXT NOT NULL,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`)

	hash, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
	db.Exec("INSERT INTO users (username, password_hash) VALUES (?, ?)", "admin", string(hash))

	t.Cleanup(func() { db.Close() })
	return db
}

func TestLogin_Success(t *testing.T) {
	db := setupTestDB(t)
	jwtSvc := NewJWTService("test-secret", 60, 168)
	h := NewHandler(db, jwtSvc)

	body, _ := json.Marshal(loginRequest{Username: "admin", Password: "password123"})
	req := httptest.NewRequest("POST", "/api/auth/login", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	h.Login(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var pair TokenPair
	json.NewDecoder(rr.Body).Decode(&pair)
	if pair.AccessToken == "" || pair.RefreshToken == "" {
		t.Fatal("expected token pair")
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	db := setupTestDB(t)
	jwtSvc := NewJWTService("test-secret", 60, 168)
	h := NewHandler(db, jwtSvc)

	body, _ := json.Marshal(loginRequest{Username: "admin", Password: "wrong"})
	req := httptest.NewRequest("POST", "/api/auth/login", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	h.Login(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestLogin_NonexistentUser(t *testing.T) {
	db := setupTestDB(t)
	jwtSvc := NewJWTService("test-secret", 60, 168)
	h := NewHandler(db, jwtSvc)

	body, _ := json.Marshal(loginRequest{Username: "nobody", Password: "password123"})
	req := httptest.NewRequest("POST", "/api/auth/login", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	h.Login(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestRefresh_Success(t *testing.T) {
	db := setupTestDB(t)
	jwtSvc := NewJWTService("test-secret", 60, 168)
	h := NewHandler(db, jwtSvc)

	pair, _ := jwtSvc.GenerateTokenPair(1, "admin")

	body, _ := json.Marshal(refreshRequest{RefreshToken: pair.RefreshToken})
	req := httptest.NewRequest("POST", "/api/auth/refresh", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	h.Refresh(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var newPair TokenPair
	json.NewDecoder(rr.Body).Decode(&newPair)
	if newPair.AccessToken == "" || newPair.RefreshToken == "" {
		t.Fatal("expected new token pair")
	}
}

func TestRefresh_WithAccessToken(t *testing.T) {
	db := setupTestDB(t)
	jwtSvc := NewJWTService("test-secret", 60, 168)
	h := NewHandler(db, jwtSvc)

	pair, _ := jwtSvc.GenerateTokenPair(1, "admin")

	body, _ := json.Marshal(refreshRequest{RefreshToken: pair.AccessToken})
	req := httptest.NewRequest("POST", "/api/auth/refresh", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	h.Refresh(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestSeedAdmin(t *testing.T) {
	db := setupTestDB(t)

	// Already has one user, so seed should be no-op
	err := SeedAdmin(db, "newpassword")
	if err != nil {
		t.Fatal(err)
	}

	var count int
	db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 user, got %d", count)
	}
}

func TestSeedAdmin_EmptyTable(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	db, _ := sql.Open("sqlite", dbPath)

	db.Exec(`CREATE TABLE users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT NOT NULL UNIQUE,
		password_hash TEXT NOT NULL,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`)
	t.Cleanup(func() { db.Close() })

	err := SeedAdmin(db, "mypassword")
	if err != nil {
		t.Fatal(err)
	}

	var count int
	db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 user, got %d", count)
	}

	var hash string
	db.QueryRow("SELECT password_hash FROM users WHERE username = 'admin'").Scan(&hash)
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte("mypassword")); err != nil {
		t.Error("password hash does not match")
	}
}
