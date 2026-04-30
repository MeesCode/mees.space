package images

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

var (
	ErrInvalidType = errors.New("invalid file type")
	ErrTooLarge    = errors.New("file too large")
	ErrNotFound    = errors.New("image not found")
)

var allowedMIME = map[string]bool{
	"image/jpeg":    true,
	"image/png":     true,
	"image/gif":     true,
	"image/webp":    true,
	"image/svg+xml": true,
}

type ImageInfo struct {
	Filename   string    `json:"filename"`
	URL        string    `json:"url"`
	Size       int64     `json:"size"`
	RefCount   int       `json:"ref_count"`
	UploadedAt time.Time `json:"uploaded_at"`
}

type Service struct {
	uploadsDir string
}

func NewService(uploadsDir string) *Service {
	return &Service{uploadsDir: uploadsDir}
}

func (s *Service) Upload(file multipart.File, header *multipart.FileHeader) (*ImageInfo, error) {
	buf := make([]byte, 512)
	n, err := file.Read(buf)
	if err != nil && err != io.EOF {
		return nil, fmt.Errorf("read file header: %w", err)
	}

	mimeType := http.DetectContentType(buf[:n])
	if !allowedMIME[mimeType] {
		// Also check file extension for SVGs since DetectContentType may return text/xml
		ext := strings.ToLower(filepath.Ext(header.Filename))
		if ext != ".svg" {
			return nil, ErrInvalidType
		}
	}

	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("seek file: %w", err)
	}

	ext := filepath.Ext(header.Filename)
	filename := fmt.Sprintf("%d_%s", time.Now().Unix(), sanitizeFilename(strings.TrimSuffix(header.Filename, ext))+ext)

	destPath := filepath.Join(s.uploadsDir, filename)
	dest, err := os.Create(destPath)
	if err != nil {
		return nil, fmt.Errorf("create file: %w", err)
	}
	defer dest.Close()

	if _, err := io.Copy(dest, file); err != nil {
		os.Remove(destPath)
		return nil, fmt.Errorf("copy file: %w", err)
	}

	info, err := os.Stat(destPath)
	if err != nil {
		return nil, err
	}

	return &ImageInfo{
		Filename:   filename,
		URL:        "/uploads/" + filename,
		Size:       info.Size(),
		RefCount:   0,
		UploadedAt: uploadedAtFromName(filename, info.ModTime()),
	}, nil
}

func (s *Service) List(refs map[string][]string) ([]ImageInfo, error) {
	entries, err := os.ReadDir(s.uploadsDir)
	if err != nil {
		return nil, fmt.Errorf("read uploads dir: %w", err)
	}

	var images []ImageInfo
	for _, e := range entries {
		if e.IsDir() || e.Name() == ".gitkeep" {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		count := 0
		if refs != nil {
			count = len(refs[e.Name()])
		}
		images = append(images, ImageInfo{
			Filename:   e.Name(),
			URL:        "/uploads/" + e.Name(),
			Size:       info.Size(),
			RefCount:   count,
			UploadedAt: uploadedAtFromName(e.Name(), info.ModTime()),
		})
	}

	return images, nil
}

func (s *Service) Delete(filename string) error {
	if strings.Contains(filename, "/") || strings.Contains(filename, "\\") || strings.Contains(filename, "..") {
		return ErrNotFound
	}

	path := filepath.Join(s.uploadsDir, filename)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return ErrNotFound
	}

	return os.Remove(path)
}

// uploadedAtFromName parses the leading "<unix-ts>_" produced by Service.Upload.
// Falls back to the supplied modTime for filenames without that prefix or with
// a non-numeric prefix.
func uploadedAtFromName(name string, modTime time.Time) time.Time {
	idx := -1
	for i, r := range name {
		if r == '_' {
			idx = i
			break
		}
	}
	if idx <= 0 {
		return modTime
	}
	secs, err := strconv.ParseInt(name[:idx], 10, 64)
	if err != nil {
		return modTime
	}
	return time.Unix(secs, 0).UTC()
}

// Refs walks contentDir for .md files and returns a map from each upload
// filename to the page paths whose contents contain that filename as a
// substring. Drafts and unpublished pages are scanned just like any other
// .md file. The map always has an entry for every non-.gitkeep file in
// the uploads directory; unused images map to an empty slice.
//
// On a per-file read error the file is skipped and the first such error is
// returned alongside the (still useful) partial map. Callers that need a
// fully trustworthy result should treat any non-nil error as "unknown".
func (s *Service) Refs(contentDir string) (map[string][]string, error) {
	entries, err := os.ReadDir(s.uploadsDir)
	if err != nil {
		return nil, fmt.Errorf("read uploads dir: %w", err)
	}

	out := make(map[string][]string, len(entries))
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || e.Name() == ".gitkeep" {
			continue
		}
		out[e.Name()] = nil
		names = append(names, e.Name())
	}

	if len(names) == 0 {
		return out, nil
	}

	var firstErr error
	walkErr := filepath.WalkDir(contentDir, func(path string, d fs.DirEntry, werr error) error {
		if werr != nil {
			if firstErr == nil {
				firstErr = werr
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(d.Name()), ".md") {
			return nil
		}
		body, readErr := os.ReadFile(path)
		if readErr != nil {
			if firstErr == nil {
				firstErr = readErr
			}
			return nil
		}
		rel, relErr := filepath.Rel(contentDir, path)
		if relErr != nil {
			rel = path
		}
		page := strings.TrimSuffix(filepath.ToSlash(rel), ".md")
		for _, name := range names {
			if bytes.Contains(body, []byte(name)) {
				out[name] = append(out[name], page)
			}
		}
		return nil
	})
	if firstErr == nil {
		firstErr = walkErr
	}
	return out, firstErr
}

func sanitizeFilename(name string) string {
	safe := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return '_'
	}, name)
	if safe == "" {
		return "file"
	}
	return safe
}
