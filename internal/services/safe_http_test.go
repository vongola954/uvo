package services

import (
	"net/http"
	"net/url"
	"testing"
)

func TestSafeDownloadDataURL(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/t.mp3"
	// "Hi" in base64
	err := SafeDownload("data:audio/mpeg;base64,SGk=", path, 1024)
	if err != nil {
		t.Fatal(err)
	}
}

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

func TestCheckDownloadRedirectBlocksPrivateHop(t *testing.T) {
	via := []*http.Request{mustHTTPSReq(t, "https://cdn1.suno.ai/track.mp3")}
	req := mustHTTPSReq(t, "https://169.254.169.254/latest/meta-data/")
	if err := checkDownloadRedirect(req, via); err == nil {
		t.Fatal("expected private IP blocked on redirect hop")
	}
}

func TestCheckDownloadRedirectBlocksHTTPHop(t *testing.T) {
	via := []*http.Request{mustHTTPSReq(t, "https://cdn1.suno.ai/track.mp3")}
	req, err := http.NewRequest(http.MethodGet, "http://cdn1.suno.ai/x.mp3", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := checkDownloadRedirect(req, via); err == nil {
		t.Fatal("expected only https on redirect")
	}
}

func TestCheckDownloadRedirectBlocksUnknownHost(t *testing.T) {
	via := []*http.Request{mustHTTPSReq(t, "https://cdn1.suno.ai/track.mp3")}
	req := mustHTTPSReq(t, "https://evil.example/steal")
	if err := checkDownloadRedirect(req, via); err == nil {
		t.Fatal("expected host not allowlisted on redirect")
	}
}

func TestCheckDownloadRedirectTooMany(t *testing.T) {
	via := make([]*http.Request, 5)
	for i := range via {
		via[i] = mustHTTPSReq(t, "https://cdn1.suno.ai/a")
	}
	req := mustHTTPSReq(t, "https://cdn2.suno.ai/b")
	if err := checkDownloadRedirect(req, via); err == nil {
		t.Fatal("expected too many redirects")
	}
}

func TestAssertSafeURLLinkLocalLiteral(t *testing.T) {
	u, err := url.Parse("https://169.254.169.254/")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("DOWNLOAD_ALLOW_HOSTS", "169.254.169.254")
	if err := assertSafeURL(u); err == nil {
		t.Fatal("link-local must be blocked even if allowlisted")
	}
}

func mustHTTPSReq(t *testing.T, raw string) *http.Request {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, raw, nil)
	if err != nil {
		t.Fatal(err)
	}
	return req
}
