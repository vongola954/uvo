package middleware

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	UserID string `json:"user_id"`
	jwt.RegisteredClaims
}

func IssueToken(secret, userID string, ttl time.Duration) (string, error) {
	claims := Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return t.SignedString([]byte(secret))
}

// OptionalAuth: Bearer if present; else anonymous only when ALLOW_ANON=true
func OptionalAuth(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.GetHeader("Authorization")
		if strings.HasPrefix(h, "Bearer ") {
			tokenStr := strings.TrimPrefix(h, "Bearer ")
			claims := &Claims{}
			token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
				if t.Method != jwt.SigningMethodHS256 {
				return nil, fmt.Errorf("unexpected signing method")
			}
				return []byte(secret), nil
			})
			if err == nil && token.Valid && claims.UserID != "" {
				c.Set("user_id", claims.UserID)
				c.Next()
				return
			}
		}
		if os.Getenv("ALLOW_ANON") == "true" {
			c.Set("user_id", "demo_user")
			c.Next()
			return
		}
		c.Set("user_id", "")
		c.Next()
	}
}

func RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		uid, _ := c.Get("user_id")
		if fmtUser(uid) == "" {
			AbortJSON(c, http.StatusUnauthorized, "unauthorized", "Bearer token required (or ALLOW_ANON=true for local demo)")
			return
		}
		c.Next()
	}
}

// UserID returns authenticated user id from context.
func UserID(c *gin.Context) string {
	uid, _ := c.Get("user_id")
	return fmtUser(uid)
}

func fmtUser(v interface{}) string {
	if v == nil {
		return ""
	}
	s, _ := v.(string)
	return s
}
