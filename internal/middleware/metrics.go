package middleware

import (
	"os"
	"strings"
	"sync/atomic"

	"github.com/gin-gonic/gin"
)

var (
	GenOK   atomic.Int64
	GenFail atomic.Int64
	HTTPReq atomic.Int64
)

func Metrics() gin.HandlerFunc {
	return func(c *gin.Context) {
		HTTPReq.Add(1)
		c.Next()
	}
}

// MetricsAuth: require X-Metrics-Token / ?token= when METRICS_TOKEN set;
// on public HTTPS without token → 404.
func MetricsAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		want := strings.TrimSpace(os.Getenv("METRICS_TOKEN"))
		if want != "" {
			got := c.GetHeader("X-Metrics-Token")
			if got == "" {
				got = c.Query("token")
			}
			if got != want {
				c.AbortWithStatus(401)
				return
			}
			c.Next()
			return
		}
		pub := strings.ToLower(strings.TrimSpace(os.Getenv("WEB_PUBLIC_URL")))
		if strings.HasPrefix(pub, "https://") {
			c.AbortWithStatus(404)
			return
		}
		c.Next()
	}
}

func MetricsHandler(c *gin.Context) {
	c.JSON(200, gin.H{
		"http_requests": HTTPReq.Load(),
		"gen_ok":        GenOK.Load(),
		"gen_fail":      GenFail.Load(),
	})
}

func IncGenOK()   { GenOK.Add(1) }
func IncGenFail() { GenFail.Add(1) }
