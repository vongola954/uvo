package services

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"testing"
	"time"
)

func TestValidateMaxWebAppInitData(t *testing.T) {
	token := "test-bot-token"
	auth := fmt.Sprintf("%d", time.Now().Unix())
	user := `{"id":12345,"first_name":"Max"}`
	pairs := "auth_date=" + auth + "\nuser=" + user

	mac := hmac.New(sha256.New, []byte("WebAppData"))
	_, _ = mac.Write([]byte(token))
	secret := mac.Sum(nil)
	mac2 := hmac.New(sha256.New, secret)
	_, _ = mac2.Write([]byte(pairs))
	hash := hex.EncodeToString(mac2.Sum(nil))

	initData := url.Values{}
	initData.Set("auth_date", auth)
	initData.Set("user", user)
	initData.Set("hash", hash)

	uid, err := ValidateMaxWebAppInitData(initData.Encode(), token, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if uid != "12345" {
		t.Fatalf("uid=%s", uid)
	}
	if _, err := ValidateMaxWebAppInitData(initData.Encode(), "wrong", time.Hour); err == nil {
		t.Fatal("expected bad token fail")
	}
}
