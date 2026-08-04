package services

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"uvo/internal/clients"
)

func TestEnsureMediaRootWritable(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "media")
	got := EnsureMediaRootWritable(root)
	if got != root {
		t.Fatalf("got %s want %s", got, root)
	}
	probe := filepath.Join(got, "x.txt")
	if err := os.WriteFile(probe, []byte("a"), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestSaveClipAudioBytes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.mp3")
	resp := &clients.GenerateResponse{AudioBytes: bytes.Repeat([]byte("A"), 128)}
	if err := saveClipAudio(resp, path, 1024); err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(path)
	if err != nil || st.Size() < 64 {
		t.Fatalf("stat %v size %v", err, st)
	}
}
