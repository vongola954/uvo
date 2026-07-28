package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func AbortJSON(c *gin.Context, status int, code, msg string) {
	c.AbortWithStatusJSON(status, gin.H{"error": APIError{Code: code, Message: msg}})
}

func RecoveryJSON() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if rec := recover(); rec != nil {
				logrus.Errorf("panic: %v", rec)
				AbortJSON(c, http.StatusInternalServerError, "internal_error", "internal server error")
			}
		}()
		c.Next()
	}
}
