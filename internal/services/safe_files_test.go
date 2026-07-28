package services

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSafeMediaPath(t *testing.T) {
	root := t.TempDir()
	inside := filepath.Join(root, "a.mp3")
	if err := os.WriteFile(inside, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	got, err := SafeMediaPath(root, inside)
	if err != nil {
		t.Fatal(err)
	}
	if got == "" {
		t.Fatal("empty path")
	}
	outside := filepath.Join(root, "..", "escape.mp3")
	if _, err := SafeMediaPath(root, outside); err == nil {
		t.Fatal("expected path outside media root")
	}
}
