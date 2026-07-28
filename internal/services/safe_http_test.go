package services

import "testing"

func TestSafeDownloadRejectsHTTP(t *testing.T) {
	err := SafeDownload("http://cdn1.suno.ai/x.mp3", "out.mp3", 1024)
	if err == nil || err.Error() != "only https allowed" {
		t.Fatalf("want https error, got %v", err)
	}
}

func TestSafeDownloadRejectsUnknownHost(t *testing.T) {
	err := SafeDownload("https://evil.example/x.mp3", "out.mp3", 1024)
	if err == nil {
		t.Fatal("expected host not allowlisted")
	}
}

func TestHostAllowed(t *testing.T) {
	if !hostAllowed("cdn1.suno.ai") {
		t.Fatal("cdn1.suno.ai should be allowed")
	}
	if hostAllowed("evil.com") {
		t.Fatal("evil.com should be denied")
	}
}

func TestSafeDownloadRejectsPrivateIP(t *testing.T) {
	t.Setenv("DOWNLOAD_ALLOW_HOSTS", "127.0.0.1")
	err := SafeDownload("https://127.0.0.1/x.mp3", "out.mp3", 1024)
	if err == nil || err.Error() != "private IP blocked" {
		t.Fatalf("want private IP blocked, got %v", err)
	}
}
