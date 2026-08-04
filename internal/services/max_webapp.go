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
	// Sometimes clients pass the full hash fragment including WebAppData=
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

	pairs, hash, err := parseInitPairs(initData)
	if err != nil {
		return "", err
	}

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

// parseInitPairs splits key=value&… like MAX docs (manual decode), not url.ParseQuery
// which can alter '+' and nested encodings.
func parseInitPairs(initData string) (map[string]string, string, error) {
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
		dv, err := url.QueryUnescape(v)
		if err != nil {
			dv = v
		}
		if k == "hash" {
			hashCount++
			hash = dv
			continue
		}
		out[k] = dv
	}
	if hashCount != 1 || hash == "" {
		// fallback: url.ParseQuery (encoded initData from Encode())
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
	return out, hash, nil
}
