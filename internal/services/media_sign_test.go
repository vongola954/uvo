package services

import (
	"strings"
	"testing"
)

func TestSignTrackDownloadURL(t *testing.T) {
	url := SignTrackDownloadURL(42, "test-secret")
	if !strings.HasPrefix(url, "/tracks/42/download?") {
		t.Fatalf("url=%s", url)
	}
	if !strings.Contains(url, "exp=") || !strings.Contains(url, "sig=") {
		t.Fatalf("missing params %s", url)
	}
	// parse query
	parts := strings.SplitN(url, "?", 2)
	q := map[string]string{}
	for _, kv := range strings.Split(parts[1], "&") {
		p := strings.SplitN(kv, "=", 2)
		if len(p) == 2 {
			q[p[0]] = p[1]
		}
	}
	if !VerifyTrackDownloadSig(42, "test-secret", q["exp"], q["sig"]) {
		t.Fatal("verify failed")
	}
	if VerifyTrackDownloadSig(43, "test-secret", q["exp"], q["sig"]) {
		t.Fatal("wrong id should fail")
	}
}
