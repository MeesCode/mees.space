package httputil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSanitizePath(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{"simple path", "hello", "hello", false},
		{"nested path", "foo/bar", "foo/bar", false},
		{"empty", "", "", true},
		{"absolute unix", "/etc/passwd", "", true},
		{"absolute windows", "\\windows\\system32", "", true},
		{"dot traversal", "../secret", "", true},
		{"nested traversal", "foo/../../etc", "", true},
		{"dot only", ".", "", true},
		{"cleans slashes", "foo//bar", "foo/bar", false},
		{"strips leading slash after clean", "/foo", "", true},
		{"null byte", "foo\x00bar", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SanitizePath(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("SanitizePath(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("SanitizePath(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsWithinDir(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "sub")
	os.MkdirAll(subDir, 0755)

	tests := []struct {
		name string
		path string
		dir  string
		want bool
	}{
		{"within", filepath.Join(tmpDir, "file.txt"), tmpDir, true},
		{"nested within", filepath.Join(subDir, "file.txt"), tmpDir, true},
		{"outside", "/etc/passwd", tmpDir, false},
		{"exact dir", tmpDir, tmpDir, false}, // dir itself is not "within"
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsWithinDir(tt.path, tt.dir)
			if got != tt.want {
				t.Errorf("IsWithinDir(%q, %q) = %v, want %v", tt.path, tt.dir, got, tt.want)
			}
		})
	}
}
