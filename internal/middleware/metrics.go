package middleware

import (
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

func MetricsHandler(c *gin.Context) {
	c.JSON(200, gin.H{
		"http_requests": HTTPReq.Load(),
		"gen_ok":        GenOK.Load(),
		"gen_fail":      GenFail.Load(),
	})
}

func IncGenOK()   { GenOK.Add(1) }
func IncGenFail() { GenFail.Add(1) }
