package services

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const mediaSignTTL = 48 * time.Hour

// PublicURL builds absolute URL AceData can fetch. Signs with JWT_SECRET when set.
func PublicURL(base, filename string) (string, error) {
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	if base == "" {
		return "", fmt.Errorf("WEB_PUBLIC_URL не задан — AceData не сможет скачать файл (нужен публичный HTTPS URL)")
	}
	if filename == "" || strings.Contains(filename, "..") || strings.ContainsAny(filename, "/\\") {
		return "", fmt.Errorf("bad upload filename")
	}
	path := base + "/uploads/" + filename
	secret := strings.TrimSpace(os.Getenv("JWT_SECRET"))
	if secret == "" {
		return path, nil
	}
	return SignMediaURL(path, filename, secret, mediaSignTTL)
}

// SignMediaURL appends exp+sig query params (HMAC-SHA256).
func SignMediaURL(absoluteURL, name, secret string, ttl time.Duration) (string, error) {
	if secret == "" {
		return "", fmt.Errorf("empty sign secret")
	}
	if ttl <= 0 {
		ttl = mediaSignTTL
	}
	exp := time.Now().Add(ttl).Unix()
	sig := mediaHMAC(name, exp, secret)
	sep := "?"
	if strings.Contains(absoluteURL, "?") {
		sep = "&"
	}
	return fmt.Sprintf("%s%sexp=%d&sig=%s", absoluteURL, sep, exp, sig), nil
}

// VerifyMediaSig checks exp+sig for a media basename.
func VerifyMediaSig(name, secret, expStr, sig string) bool {
	if secret == "" || name == "" || expStr == "" || sig == "" {
		return false
	}
	exp, err := strconv.ParseInt(expStr, 10, 64)
	if err != nil || exp < time.Now().Unix() {
		return false
	}
	want := mediaHMAC(name, exp, secret)
	return hmac.Equal([]byte(want), []byte(strings.ToLower(sig))) || hmac.Equal([]byte(want), []byte(sig))
}

func mediaHMAC(name string, exp int64, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(fmt.Sprintf("%s|%d", name, exp)))
	return hex.EncodeToString(mac.Sum(nil))
}

const trackDownloadTTL = time.Hour

// TrackDownloadResource is the HMAC subject for signed track downloads.
func TrackDownloadResource(trackID uint) string {
	return fmt.Sprintf("track:%d", trackID)
}

// SignTrackDownloadURL builds /tracks/{id}/download?exp=&sig= (TTL 1h).
// If JWT_SECRET empty, returns unsigned path (dev only).
func SignTrackDownloadURL(trackID uint, secret string) string {
	path := fmt.Sprintf("/tracks/%d/download", trackID)
	if secret == "" {
		return path
	}
	exp := time.Now().Add(trackDownloadTTL).Unix()
	sig := mediaHMAC(TrackDownloadResource(trackID), exp, secret)
	return fmt.Sprintf("%s?exp=%d&sig=%s", path, exp, sig)
}

// VerifyTrackDownloadSig checks signed download query params.
func VerifyTrackDownloadSig(trackID uint, secret, expStr, sig string) bool {
	return VerifyMediaSig(TrackDownloadResource(trackID), secret, expStr, sig)
}
