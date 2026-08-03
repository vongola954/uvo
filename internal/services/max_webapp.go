package services

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ValidateMaxWebAppInitData checks MAX Mini App initData (window.WebApp.initData).
// Algorithm: https://dev.max.ru/docs/webapps/validation
func ValidateMaxWebAppInitData(initData, botToken string, maxAge time.Duration) (userID string, err error) {
	initData = strings.TrimSpace(initData)
	botToken = strings.TrimSpace(botToken)
	if initData == "" || botToken == "" {
		return "", fmt.Errorf("empty initData or token")
	}
	vals, err := url.ParseQuery(initData)
	if err != nil {
		return "", fmt.Errorf("parse initData: %w", err)
	}
	hash := vals.Get("hash")
	if hash == "" {
		return "", fmt.Errorf("missing hash")
	}
	vals.Del("hash")

	pairs := make([]string, 0, len(vals))
	for k := range vals {
		pairs = append(pairs, k+"="+vals.Get(k))
	}
	sort.Strings(pairs)
	launchParams := strings.Join(pairs, "\n")

	mac := hmac.New(sha256.New, []byte("WebAppData"))
	_, _ = mac.Write([]byte(botToken))
	secret := mac.Sum(nil)

	mac2 := hmac.New(sha256.New, secret)
	_, _ = mac2.Write([]byte(launchParams))
	want := hex.EncodeToString(mac2.Sum(nil))
	if !hmac.Equal([]byte(strings.ToLower(want)), []byte(strings.ToLower(hash))) {
		return "", fmt.Errorf("invalid hash")
	}

	if maxAge > 0 {
		ad := vals.Get("auth_date")
		sec, err := strconv.ParseInt(ad, 10, 64)
		if err != nil || sec <= 0 {
			return "", fmt.Errorf("bad auth_date")
		}
		if time.Since(time.Unix(sec, 0)) > maxAge {
			return "", fmt.Errorf("initData expired")
		}
	}

	userJSON := vals.Get("user")
	if userJSON == "" {
		return "", fmt.Errorf("missing user")
	}
	var u struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal([]byte(userJSON), &u); err != nil || u.ID == 0 {
		return "", fmt.Errorf("bad user")
	}
	return strconv.FormatInt(u.ID, 10), nil
}
