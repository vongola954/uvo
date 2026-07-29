package services

import (
	"strings"
	"testing"
)

func TestPublicURL(t *testing.T) {
	t.Setenv("JWT_SECRET", "")
	u, err := PublicURL("https://example.com/", "abc.mp3")
	if err != nil || u != "https://example.com/uploads/abc.mp3" {
		t.Fatalf("unsigned got %q %v", u, err)
	}
	if _, err := PublicURL("", "abc.mp3"); err == nil {
		t.Fatal("expected error without base")
	}
	if _, err := PublicURL("https://x.com", "../x.mp3"); err == nil {
		t.Fatal("expected path traversal reject")
	}
}

func TestPublicURLSigned(t *testing.T) {
	secret := "test-secret-at-least-32-chars-long!!"
	t.Setenv("JWT_SECRET", secret)
	u, err := PublicURL("https://example.com", "abc.mp3")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(u, "exp=") || !strings.Contains(u, "sig=") {
		t.Fatalf("want signed url, got %s", u)
	}
	_, q, ok := strings.Cut(u, "?")
	if !ok {
		t.Fatal("no query")
	}
	var exp, sig string
	for _, part := range strings.Split(q, "&") {
		k, v, _ := strings.Cut(part, "=")
		switch k {
		case "exp":
			exp = v
		case "sig":
			sig = v
		}
	}
	if !VerifyMediaSig("abc.mp3", secret, exp, sig) {
		t.Fatal("signature should verify")
	}
	if VerifyMediaSig("abc.mp3", secret, "1", sig) {
		t.Fatal("expired should fail")
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
