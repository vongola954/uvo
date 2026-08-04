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
	// Full fragment: #WebAppData=...&WebAppPlatform=...
	if strings.Contains(initData, "WebAppData=") || strings.Contains(initData, "webAppData=") {
		vals, err := url.ParseQuery(strings.TrimPrefix(initData, "#"))
		if err == nil {
			if v := vals.Get("WebAppData"); v != "" {
				initData = v
			} else if v := vals.Get("webAppData"); v != "" {
				initData = v
			}
		}
	}

	candidates := []string{initData}
	if d1, e := url.QueryUnescape(initData); e == nil && d1 != initData {
		candidates = append(candidates, d1)
		if d2, e2 := url.QueryUnescape(d1); e2 == nil && d2 != d1 {
			candidates = append(candidates, d2)
		}
	}

	var last error
	for _, cand := range candidates {
		uid, err := validateMaxInitOnce(cand, botToken, maxAge)
		if err == nil {
			return uid, nil
		}
		last = err
	}
	if last == nil {
		last = fmt.Errorf("invalid initData")
	}
	return "", last
}

func validateMaxInitOnce(initData, botToken string, maxAge time.Duration) (string, error) {
	// Try: decoded values (docs) and raw values (some WebViews).
	for _, decodeVals := range []bool{true, false} {
		pairs, hash, err := parseInitPairs(initData, decodeVals)
		if err != nil {
			continue
		}
		if checkMaxHash(pairs, hash, botToken) {
			return userIDFromPairs(pairs, maxAge)
		}
	}
	return "", fmt.Errorf("invalid hash")
}

func checkMaxHash(pairs map[string]string, hash, botToken string) bool {
	keys := make([]string, 0, len(pairs))
	for k := range pairs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for i, k := range keys {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(pairs[k])
	}
	launchParams := b.String()

	// Docs: secret = HMAC_SHA256(key="WebAppData", msg=BOT_TOKEN)
	mac := hmac.New(sha256.New, []byte("WebAppData"))
	_, _ = mac.Write([]byte(botToken))
	secret := mac.Sum(nil)
	mac2 := hmac.New(sha256.New, secret)
	_, _ = mac2.Write([]byte(launchParams))
	want := hex.EncodeToString(mac2.Sum(nil))
	if hmac.Equal([]byte(strings.ToLower(want)), []byte(strings.ToLower(hash))) {
		return true
	}

	// Alternate key order seen in some samples: HMAC(key=token, msg="WebAppData")
	mac = hmac.New(sha256.New, []byte(botToken))
	_, _ = mac.Write([]byte("WebAppData"))
	secret = mac.Sum(nil)
	mac2 = hmac.New(sha256.New, secret)
	_, _ = mac2.Write([]byte(launchParams))
	want = hex.EncodeToString(mac2.Sum(nil))
	return hmac.Equal([]byte(strings.ToLower(want)), []byte(strings.ToLower(hash)))
}

func userIDFromPairs(pairs map[string]string, maxAge time.Duration) (string, error) {
	if maxAge > 0 {
		ad := pairs["auth_date"]
		sec, err := strconv.ParseInt(ad, 10, 64)
		if err != nil || sec <= 0 {
			return "", fmt.Errorf("bad auth_date")
		}
		if time.Since(time.Unix(sec, 0)) > maxAge {
			return "", fmt.Errorf("initData expired")
		}
	}
	userJSON := pairs["user"]
	if userJSON == "" {
		return "", fmt.Errorf("missing user")
	}
	var u struct {
		ID     int64 `json:"id"`
		UserID int64 `json:"user_id"`
	}
	if err := json.Unmarshal([]byte(userJSON), &u); err != nil {
		return "", fmt.Errorf("bad user: %w", err)
	}
	id := u.ID
	if id == 0 {
		id = u.UserID
	}
	if id == 0 {
		return "", fmt.Errorf("bad user id")
	}
	return strconv.FormatInt(id, 10), nil
}

func parseInitPairs(initData string, decodeVals bool) (map[string]string, string, error) {
	parts := strings.Split(initData, "&")
	out := make(map[string]string, len(parts))
	hash := ""
	hashCount := 0
	for _, p := range parts {
		if p == "" {
			continue
		}
		k, v, ok := strings.Cut(p, "=")
		if !ok {
			k, v = p, ""
		}
		if k == "" {
			continue
		}
		dv := v
		if decodeVals {
			if d, err := url.QueryUnescape(v); err == nil {
				dv = d
			}
		}
		if k == "hash" {
			hashCount++
			hash = dv
			continue
		}
		out[k] = dv
	}
	if hashCount == 1 && hash != "" {
		return out, hash, nil
	}
	vals, err := url.ParseQuery(initData)
	if err != nil {
		return nil, "", fmt.Errorf("missing hash")
	}
	hash = vals.Get("hash")
	if hash == "" {
		return nil, "", fmt.Errorf("missing hash")
	}
	vals.Del("hash")
	out = make(map[string]string, len(vals))
	for k := range vals {
		out[k] = vals.Get(k)
	}
	return out, hash, nil
}
