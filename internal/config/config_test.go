package config

import (
	"os"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	os.Setenv("MEES_JWT_SECRET", "test-secret")
	defer os.Unsetenv("MEES_JWT_SECRET")

	// Clear any overrides
	os.Unsetenv("MEES_PORT")
	os.Unsetenv("MEES_DATABASE_PATH")
	os.Unsetenv("MEES_JWT_EXPIRY_MINUTES")
	os.Unsetenv("MEES_JWT_REFRESH_EXPIRY_HOURS")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Port != "8080" {
		t.Errorf("Port = %q, want %q", cfg.Port, "8080")
	}
	if cfg.DatabasePath != "./mees.db" {
		t.Errorf("DatabasePath = %q, want %q", cfg.DatabasePath, "./mees.db")
	}
	if cfg.JWTExpiryMinutes != 60 {
		t.Errorf("JWTExpiryMinutes = %d, want %d", cfg.JWTExpiryMinutes, 60)
	}
	if cfg.JWTRefreshExpiryHrs != 168 {
		t.Errorf("JWTRefreshExpiryHrs = %d, want %d", cfg.JWTRefreshExpiryHrs, 168)
	}
}

func TestLoadCustomValues(t *testing.T) {
	os.Setenv("MEES_JWT_SECRET", "my-secret")
	os.Setenv("MEES_PORT", "9090")
	os.Setenv("MEES_JWT_EXPIRY_MINUTES", "30")
	os.Setenv("MEES_JWT_REFRESH_EXPIRY_HOURS", "24")
	defer func() {
		os.Unsetenv("MEES_JWT_SECRET")
		os.Unsetenv("MEES_PORT")
		os.Unsetenv("MEES_JWT_EXPIRY_MINUTES")
		os.Unsetenv("MEES_JWT_REFRESH_EXPIRY_HOURS")
	}()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.JWTSecret != "my-secret" {
		t.Errorf("JWTSecret = %q, want %q", cfg.JWTSecret, "my-secret")
	}
	if cfg.Port != "9090" {
		t.Errorf("Port = %q, want %q", cfg.Port, "9090")
	}
	if cfg.JWTExpiryMinutes != 30 {
		t.Errorf("JWTExpiryMinutes = %d, want %d", cfg.JWTExpiryMinutes, 30)
	}
	if cfg.JWTRefreshExpiryHrs != 24 {
		t.Errorf("JWTRefreshExpiryHrs = %d, want %d", cfg.JWTRefreshExpiryHrs, 24)
	}
}

func TestLoadMissingJWTSecret(t *testing.T) {
	os.Unsetenv("MEES_JWT_SECRET")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() should fail when MEES_JWT_SECRET is missing")
	}
}

func TestLoadInvalidIntFallsBackToDefault(t *testing.T) {
	os.Setenv("MEES_JWT_SECRET", "test")
	os.Setenv("MEES_JWT_EXPIRY_MINUTES", "not-a-number")
	defer func() {
		os.Unsetenv("MEES_JWT_SECRET")
		os.Unsetenv("MEES_JWT_EXPIRY_MINUTES")
	}()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.JWTExpiryMinutes != 60 {
		t.Errorf("JWTExpiryMinutes = %d, want default 60", cfg.JWTExpiryMinutes)
	}
}
