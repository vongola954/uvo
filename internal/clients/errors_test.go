package clients

import "testing"

func TestParseAceDataUsedUp(t *testing.T) {
	body := []byte(`{"error":{"message":"Your balance is not sufficient","code":"used_up"}}`)
	err := ParseAceDataHTTPError(403, body)
	pe, ok := err.(*ProviderError)
	if !ok {
		t.Fatal("expected ProviderError")
	}
	if pe.Code != "provider_balance_empty" {
		t.Fatalf("code %s", pe.Code)
	}
	if pe.Status != 503 {
		t.Fatalf("status %d", pe.Status)
	}
}

func TestAsProviderErrorWrap(t *testing.T) {
	err := ParseAceDataHTTPError(403, []byte(`{"error":{"code":"used_up","message":"x"}}`))
	if AsProviderError(err) == nil {
		t.Fatal("expected as")
	}
}
