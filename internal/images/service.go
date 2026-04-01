package images

import (
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
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
	Filename string `json:"filename"`
	URL      string `json:"url"`
	Size     int64  `json:"size"`
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
		Filename: filename,
		URL:      "/uploads/" + filename,
		Size:     info.Size(),
	}, nil
}

func (s *Service) List() ([]ImageInfo, error) {
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
		images = append(images, ImageInfo{
			Filename: e.Name(),
			URL:      "/uploads/" + e.Name(),
			Size:     info.Size(),
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
