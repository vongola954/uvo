package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

const csrfCookie = "uvo_csrf"
const csrfHeader = "X-CSRF-Token"

func newToken() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func csrfSecure() bool {
	u := strings.ToLower(os.Getenv("WEB_PUBLIC_URL"))
	return strings.HasPrefix(u, "https://")
}

func needsCSRF(method, path string) bool {
	if method == http.MethodGet || method == http.MethodHead || method == http.MethodOptions {
		return false
	}
	prefixes := []string{
		"/api/feed",
		"/api/playlists",
		"/api/generate",
		"/api/voice",
		"/api/tts",
		"/api/cover",
		"/api/credits/topup",
		"/api/bot/simulate",
	}
	for _, p := range prefixes {
		if strings.HasPrefix(path, p) {
			return true
		}
	}
	if strings.HasPrefix(path, "/api/tracks") && (method == http.MethodDelete || method == http.MethodPatch ||
		strings.Contains(path, "/edit") || strings.Contains(path, "/visibility")) {
		return true
	}
	return false
}

// CSRF issues cookie token on safe methods; validates header on mutating /api/*.
func CSRF() gin.HandlerFunc {
	return func(c *gin.Context) {
		tok, err := c.Cookie(csrfCookie)
		if err != nil || tok == "" {
			tok = newToken()
			c.SetCookie(csrfCookie, tok, 86400, "/", "", csrfSecure(), true)
		}
		c.Header(csrfHeader, tok)

		if needsCSRF(c.Request.Method, c.Request.URL.Path) {
			got := c.GetHeader(csrfHeader)
			if got == "" || got != tok {
				AbortJSON(c, http.StatusForbidden, "csrf_rejected", "invalid csrf token")
				return
			}
		}
		c.Next()
	}
}
