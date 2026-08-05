package clients

import (
	"testing"
)

func TestRussianTrustedPoolLoads(t *testing.T) {
	pool, err := russianTrustedPool()
	if err != nil {
		t.Fatal(err)
	}
	if pool == nil {
		t.Fatal("nil pool")
	}
	c := HTTPClient(5)
	if c == nil || c.Transport == nil {
		t.Fatal("nil client/transport")
	}
}
