package middleware

import (
	"crypto/sha256"
	"crypto/subtle"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

const SessionCookie = "uvo_session"

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

func parseToken(secret, tokenStr string) (string, bool) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return []byte(secret), nil
	})
	if err != nil || !token.Valid || claims.UserID == "" {
		return "", false
	}
	return claims.UserID, true
}

func cookieSecure() bool {
	u := strings.ToLower(os.Getenv("WEB_PUBLIC_URL"))
	return strings.HasPrefix(u, "https://")
}

// SetSessionCookie issues a 7-day HttpOnly session JWT cookie.
func SetSessionCookie(c *gin.Context, secret, userID string) error {
	tok, err := IssueToken(secret, userID, 7*24*time.Hour)
	if err != nil {
		return err
	}
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(SessionCookie, tok, int((7 * 24 * time.Hour).Seconds()), "/", "", cookieSecure(), true)
	return nil
}

func ClearSessionCookie(c *gin.Context) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(SessionCookie, "", -1, "/", "", cookieSecure(), true)
}

// OptionalAuth: Bearer, then uvo_session cookie; else anonymous only when ALLOW_ANON=true
func OptionalAuth(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.GetHeader("Authorization")
		if strings.HasPrefix(h, "Bearer ") {
			tokenStr := strings.TrimPrefix(h, "Bearer ")
			if uid, ok := parseToken(secret, tokenStr); ok {
				c.Set("user_id", uid)
				c.Next()
				return
			}
		}
		if cookie, err := c.Cookie(SessionCookie); err == nil && cookie != "" {
			if uid, ok := parseToken(secret, cookie); ok {
				c.Set("user_id", uid)
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
			AbortJSON(c, http.StatusUnauthorized, "unauthorized", "Нужна авторизация: MAX /login")
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

// SecretEqual compares secrets in constant time (length-independent via SHA-256).
func SecretEqual(got, want string) bool {
	a := sha256.Sum256([]byte(got))
	b := sha256.Sum256([]byte(want))
	return subtle.ConstantTimeCompare(a[:], b[:]) == 1
}
