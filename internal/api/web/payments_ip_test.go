package web

import (
	"testing"
)

func TestYooWebhookIPAllowed(t *testing.T) {
	t.Setenv("YOOKASSA_WEBHOOK_IPS", "")
	if !yooWebhookIPAllowed("1.2.3.4") {
		t.Fatal("empty allowlist should allow")
	}
	t.Setenv("YOOKASSA_WEBHOOK_IPS", "185.71.76.0/27,1.2.3.4")
	if !yooWebhookIPAllowed("1.2.3.4") {
		t.Fatal("exact IP")
	}
	if !yooWebhookIPAllowed("185.71.76.1") {
		t.Fatal("CIDR")
	}
	if yooWebhookIPAllowed("8.8.8.8") {
		t.Fatal("outside allowlist")
	}
}
