package middleware

import (
	"crypto/subtle"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

const maxWebhookSecretHeader = "X-Max-Bot-Api-Secret"

// MaxWebhookAuth validates MAX_WEBHOOK_SECRET via header or ?secret=.
// Required whenever the webhook endpoint is hit; BOT_MODE=webhook also requires the env to be set.
func MaxWebhookAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		secret := os.Getenv("MAX_WEBHOOK_SECRET")
		if secret == "" {
			AbortJSON(c, http.StatusForbidden, "forbidden", "MAX_WEBHOOK_SECRET not configured")
			return
		}
		got := c.GetHeader(maxWebhookSecretHeader)
		if got == "" {
			got = c.Query("secret")
		}
		if subtle.ConstantTimeCompare([]byte(got), []byte(secret)) != 1 {
			AbortJSON(c, http.StatusUnauthorized, "unauthorized", "invalid webhook secret")
			return
		}
		c.Next()
	}
}
