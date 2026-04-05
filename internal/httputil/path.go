package httputil

import (
	"errors"
	"path/filepath"
	"strings"
)

var ErrInvalidPath = errors.New("invalid path")

func SanitizePath(raw string) (string, error) {
	if raw == "" {
		return "", ErrInvalidPath
	}

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

func IsWithinDir(path, dir string) bool {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return false
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	return strings.HasPrefix(absPath, absDir+string(filepath.Separator)) || strings.HasPrefix(absPath, absDir+"/")
}
