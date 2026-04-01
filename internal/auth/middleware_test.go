package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequireAuth_ValidToken(t *testing.T) {
	svc := NewJWTService("test-secret", 60, 168)
	pair, _ := svc.GenerateTokenPair(1, "admin")

	var gotUser *UserInfo
	handler := RequireAuth(svc, func(w http.ResponseWriter, r *http.Request) {
		gotUser = GetUser(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+pair.AccessToken)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
	if gotUser == nil {
		t.Fatal("expected user in context")
	}
	if gotUser.ID != 1 {
		t.Errorf("expected user ID 1, got %d", gotUser.ID)
	}
	if gotUser.Username != "admin" {
		t.Errorf("expected username admin, got %s", gotUser.Username)
	}
}

func TestRequireAuth_MissingHeader(t *testing.T) {
	svc := NewJWTService("test-secret", 60, 168)

	handler := RequireAuth(svc, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestRequireAuth_InvalidToken(t *testing.T) {
	svc := NewJWTService("test-secret", 60, 168)

	handler := RequireAuth(svc, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestRequireAuth_RefreshTokenRejected(t *testing.T) {
	svc := NewJWTService("test-secret", 60, 168)
	pair, _ := svc.GenerateTokenPair(1, "admin")

	handler := RequireAuth(svc, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+pair.RefreshToken)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestRequireAuth_MalformedHeader(t *testing.T) {
	svc := NewJWTService("test-secret", 60, 168)

	handler := RequireAuth(svc, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "NotBearer some-token")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}
