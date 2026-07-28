package clients

import (
	"strings"
	"testing"
)

func TestRedactBody(t *testing.T) {
	short := []byte(`{"ok":true}`)
	if got := redactBody(short, 256); got != string(short) {
		t.Fatalf("short: %q", got)
	}
	long := make([]byte, 400)
	for i := range long {
		long[i] = 'a'
	}
	got := redactBody(long, 32)
	if !strings.HasSuffix(got, "…[redacted]") {
		t.Fatalf("missing redacted marker: %q", got)
	}
}

func TestParseAceDataRateLimit(t *testing.T) {
	err := ParseAceDataHTTPError(429, []byte(`{"error":{"message":"rate limited"}}`))
	pe, ok := err.(*ProviderError)
	if !ok {
		t.Fatal("expected ProviderError")
	}
	if pe.Code != "provider_rate_limit" || pe.Status != 429 {
		t.Fatalf("code=%s status=%d", pe.Code, pe.Status)
	}
}
