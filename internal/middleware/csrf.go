package middleware

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
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
	return cookieSecure()
}

func needsCSRF(method, path string) bool {
	if method == http.MethodGet || method == http.MethodHead || method == http.MethodOptions {
		return false
	}
	if path == "/api/auth/exchange" || path == "/api/auth/logout" || path == "/api/auth/max-webapp" {
		return false
	}
	if !strings.HasPrefix(path, "/api/") {
		return false
	}
	// All mutating /api/* except webhook (uses its own secret)
	if path == "/api/max/webhook" || path == "/api/auth/token" || path == "/api/payments/yookassa" {
		return false
	}
	return true
}

// CSRF issues readable cookie (double-submit) + SameSite; validates header on mutate.
func CSRF() gin.HandlerFunc {
	return func(c *gin.Context) {
		tok, err := c.Cookie(csrfCookie)
		if err != nil || tok == "" {
			tok = newToken()
			// Lax: MAX WebView / mini-app navigations must still receive CSRF cookie
			c.SetSameSite(http.SameSiteLaxMode)
			// HttpOnly=false: JS must read cookie for X-CSRF-Token header
			c.SetCookie(csrfCookie, tok, 86400, "/", "", csrfSecure(), false)
		}
		c.Header(csrfHeader, tok)

		if needsCSRF(c.Request.Method, c.Request.URL.Path) {
			got := c.GetHeader(csrfHeader)
			if got == "" || subtle.ConstantTimeCompare([]byte(got), []byte(tok)) != 1 {
				AbortJSON(c, http.StatusForbidden, "csrf_rejected", "invalid csrf token")
				return
			}
		}
		c.Next()
	}
}
