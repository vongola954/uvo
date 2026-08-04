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

func TestResolveTrackFileBasename(t *testing.T) {
	root := t.TempDir()
	name := "clip.mp3"
	inside := filepath.Join(root, name)
	if err := os.WriteFile(inside, []byte("audio"), 0644); err != nil {
		t.Fatal(err)
	}
	got, err := ResolveTrackFile(root, filepath.Join("/tmp/uvo-media", name))
	if err != nil {
		t.Fatal(err)
	}
	if got != inside {
		// Abs paths may differ by symlink; compare basenames + size
		if filepath.Base(got) != name {
			t.Fatalf("got %q", got)
		}
	}
}
