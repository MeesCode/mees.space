package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	Port                string
	DatabasePath        string
	ContentDir          string
	UploadsDir          string
	DistDir             string
	JWTSecret           string
	JWTExpiryMinutes    int
	JWTRefreshExpiryHrs int
	AdminPassword       string
}

func Load() (*Config, error) {
	cfg := &Config{
		Port:                envOr("MEES_PORT", "8080"),
		DatabasePath:        envOr("MEES_DATABASE_PATH", "./mees.db"),
		ContentDir:          envOr("MEES_CONTENT_DIR", "./content"),
		UploadsDir:          envOr("MEES_UPLOADS_DIR", "./uploads"),
		DistDir:             envOr("MEES_DIST_DIR", "./dist"),
		JWTSecret:           os.Getenv("MEES_JWT_SECRET"),
		JWTExpiryMinutes:    envOrInt("MEES_JWT_EXPIRY_MINUTES", 60),
		JWTRefreshExpiryHrs: envOrInt("MEES_JWT_REFRESH_EXPIRY_HOURS", 168),
		AdminPassword:       os.Getenv("MEES_ADMIN_PASSWORD"),
	}

	if cfg.JWTSecret == "" {
		return nil, fmt.Errorf("MEES_JWT_SECRET environment variable is required")
	}

	return cfg, nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envOrInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}
