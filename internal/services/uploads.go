package services

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

// PublicUpload saves bytes under mediaRoot/uploads and returns relative name + absolute path.
func PublicUpload(mediaRoot string, data []byte, ext string) (filename, absPath string, err error) {
	if mediaRoot == "" {
		mediaRoot = "./data/media"
	}
	ext = strings.ToLower(strings.TrimSpace(ext))
	if ext == "" {
		ext = ".mp3"
	}
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	switch ext {
	case ".mp3", ".wav", ".m4a", ".ogg":
	default:
		ext = ".mp3"
	}
	dir := filepath.Join(mediaRoot, "uploads")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", "", err
	}
	filename = uuid.New().String() + ext
	absPath = filepath.Join(dir, filename)
	if err := os.WriteFile(absPath, data, 0644); err != nil {
		return "", "", err
	}
	return filename, absPath, nil
}

// PublicURL builds absolute URL AceData can fetch. Requires WEB_PUBLIC_URL / cfg.
func PublicURL(base, filename string) (string, error) {
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	if base == "" {
		return "", fmt.Errorf("WEB_PUBLIC_URL не задан — AceData не сможет скачать файл (нужен публичный HTTPS URL)")
	}
	if filename == "" || strings.Contains(filename, "..") || strings.ContainsAny(filename, "/\\") {
		return "", fmt.Errorf("bad upload filename")
	}
	return base + "/uploads/" + filename, nil
}

// ResolveUploadPath maps /uploads/:name to disk path under mediaRoot/uploads.
func ResolveUploadPath(mediaRoot, name string) (string, error) {
	if mediaRoot == "" {
		mediaRoot = "./data/media"
	}
	name = filepath.Base(name)
	if name == "" || name == "." || name == ".." {
		return "", fmt.Errorf("invalid name")
	}
	raw := filepath.Join(mediaRoot, "uploads", name)
	return SafeMediaPath(filepath.Join(mediaRoot, "uploads"), raw)
}
