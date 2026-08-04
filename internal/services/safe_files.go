package services

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// SafeJoin ensures path is under allowed root (project 1.0 Epoch 4)
func SafeMediaPath(mediaRoot, filePath string) (string, error) {
	root, err := filepath.Abs(mediaRoot)
	if err != nil {
		return "", err
	}
	abs, err := filepath.Abs(filePath)
	if err != nil {
		return "", err
	}
	if !pathUnderRoot(abs, root) {
		return "", fmt.Errorf("path outside media root")
	}
	return abs, nil
}

func pathUnderRoot(abs, root string) bool {
	sep := string(os.PathSeparator)
	return abs == root || strings.HasPrefix(abs, root+sep)
}

// ResolveTrackFile returns a readable media path for a stored track FilePath.
// Accepts files under MEDIA_ROOT, basename lookup, and legacy /tmp/uvo-media fallback.
func ResolveTrackFile(mediaRoot, filePath string) (string, error) {
	filePath = strings.TrimSpace(filePath)
	if filePath == "" {
		return "", fmt.Errorf("empty file path")
	}
	try := func(p string) (string, bool) {
		if p == "" {
			return "", false
		}
		st, err := os.Stat(p)
		if err != nil || st.IsDir() {
			return "", false
		}
		return p, true
	}
	if safe, err := SafeMediaPath(mediaRoot, filePath); err == nil {
		if p, ok := try(safe); ok {
			return p, nil
		}
	}
	base := filepath.Base(filePath)
	if base != "" && base != "." && !strings.Contains(base, "..") {
		cand := filepath.Join(mediaRoot, base)
		if safe, err := SafeMediaPath(mediaRoot, cand); err == nil {
			if p, ok := try(safe); ok {
				return p, nil
			}
		}
	}
	// Legacy fallback root used when /data/media was not writable.
	clean := filepath.Clean(filePath)
	if strings.HasPrefix(clean, "/tmp/uvo-media"+string(os.PathSeparator)) || clean == "/tmp/uvo-media" {
		if p, ok := try(clean); ok {
			return p, nil
		}
	}
	return "", fmt.Errorf("media file missing")
}
