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
	if !strings.HasPrefix(abs, root+string(os.PathSeparator)) && abs != root {
		return "", fmt.Errorf("path outside media root")
	}
	return abs, nil
}
