package services

import "testing"

func TestPublicURL(t *testing.T) {
	u, err := PublicURL("https://example.com/", "abc.mp3")
	if err != nil || u != "https://example.com/uploads/abc.mp3" {
		t.Fatalf("got %q %v", u, err)
	}
	if _, err := PublicURL("", "abc.mp3"); err == nil {
		t.Fatal("expected error without base")
	}
	if _, err := PublicURL("https://x.com", "../x.mp3"); err == nil {
		t.Fatal("expected path traversal reject")
	}
}

func TestResolveUploadPath(t *testing.T) {
	root := t.TempDir()
	filename, abs, err := PublicUpload(root, []byte("hello-audio-bytes"), ".mp3")
	if err != nil {
		t.Fatal(err)
	}
	got, err := ResolveUploadPath(root, filename)
	if err != nil {
		t.Fatal(err)
	}
	if got != abs {
		t.Fatalf("%s != %s", got, abs)
	}
	// filepath.Base strips ".."; remaining name must still stay under uploads/
	got2, err := ResolveUploadPath(root, "../"+filename)
	if err != nil {
		t.Fatal(err)
	}
	if got2 != abs {
		t.Fatalf("base escape should still resolve to same file: %s", got2)
	}
	if _, err := ResolveUploadPath(root, "."); err == nil {
		t.Fatal("expected reject for '.'")
	}
}
