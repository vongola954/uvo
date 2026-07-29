package middleware

import (
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

const maxWebhookSecretHeader = "X-Max-Bot-Api-Secret"

// MaxWebhookAuth validates MAX_WEBHOOK_SECRET via header only (no query secret).
func MaxWebhookAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		secret := os.Getenv("MAX_WEBHOOK_SECRET")
		if secret == "" {
			AbortJSON(c, http.StatusForbidden, "forbidden", "MAX_WEBHOOK_SECRET not configured")
			return
		}
		got := c.GetHeader(maxWebhookSecretHeader)
		if !SecretEqual(got, secret) {
			AbortJSON(c, http.StatusUnauthorized, "unauthorized", "invalid webhook secret")
			return
		}
		c.Next()
	}
}
