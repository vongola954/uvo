package middleware

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestRequireAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	os.Unsetenv("ALLOW_ANON")

	r := gin.New()
	r.Use(OptionalAuth("test-secret"), RequireAuth())
	r.GET("/x", func(c *gin.Context) { c.Status(200) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	r.ServeHTTP(w, req)
	if w.Code != 401 {
		t.Fatalf("want 401, got %d", w.Code)
	}

	os.Setenv("ALLOW_ANON", "true")
	defer os.Unsetenv("ALLOW_ANON")
	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/x", nil)
	r.ServeHTTP(w2, req2)
	if w2.Code != 200 {
		t.Fatalf("ALLOW_ANON want 200, got %d", w2.Code)
	}
}

func TestMaxWebhookAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	os.Unsetenv("MAX_WEBHOOK_SECRET")

	r := gin.New()
	r.POST("/hook", MaxWebhookAuth(), func(c *gin.Context) { c.Status(200) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/hook", nil)
	r.ServeHTTP(w, req)
	if w.Code != 403 {
		t.Fatalf("no secret configured want 403, got %d", w.Code)
	}

	os.Setenv("MAX_WEBHOOK_SECRET", "s3cret")
	defer os.Unsetenv("MAX_WEBHOOK_SECRET")

	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodPost, "/hook", nil)
	r.ServeHTTP(w2, req2)
	if w2.Code != 401 {
		t.Fatalf("wrong secret want 401, got %d", w2.Code)
	}

	w3 := httptest.NewRecorder()
	req3 := httptest.NewRequest(http.MethodPost, "/hook", nil)
	req3.Header.Set("X-Max-Bot-Api-Secret", "s3cret")
	r.ServeHTTP(w3, req3)
	if w3.Code != 200 {
		t.Fatalf("valid secret want 200, got %d", w3.Code)
	}

	// Query secret must NOT work (epoch 14)
	w4 := httptest.NewRecorder()
	req4 := httptest.NewRequest(http.MethodPost, "/hook?secret=s3cret", nil)
	r.ServeHTTP(w4, req4)
	if w4.Code != 401 {
		t.Fatalf("query secret must be rejected, got %d", w4.Code)
	}
}

func TestOptionalAuthSessionCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)
	os.Unsetenv("ALLOW_ANON")
	secret := "jwt-cookie-secret-for-tests!!"

	r := gin.New()
	r.POST("/login", func(c *gin.Context) {
		if err := SetSessionCookie(c, secret, "cookie-user"); err != nil {
			c.Status(500)
			return
		}
		c.Status(200)
	})
	r.GET("/me", OptionalAuth(secret), RequireAuth(), func(c *gin.Context) {
		c.JSON(200, gin.H{"uid": UserID(c)})
	})

	warm := httptest.NewRecorder()
	r.ServeHTTP(warm, httptest.NewRequest(http.MethodPost, "/login", nil))
	if warm.Code != 200 {
		t.Fatalf("login want 200, got %d", warm.Code)
	}
	var sess string
	for _, c := range warm.Result().Cookies() {
		if c.Name == SessionCookie {
			sess = c.Value
		}
	}
	if sess == "" {
		t.Fatal("expected session cookie")
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	req.AddCookie(&http.Cookie{Name: SessionCookie, Value: sess})
	r.ServeHTTP(w, req)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "cookie-user") {
		t.Fatalf("cookie auth failed: %d %s", w.Code, w.Body.String())
	}
}

func TestOptionalAuthJWT(t *testing.T) {
	gin.SetMode(gin.TestMode)
	os.Unsetenv("ALLOW_ANON")

	secret := "jwt-test-secret"
	tok, err := IssueToken(secret, "user-42", time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	r := gin.New()
	r.Use(OptionalAuth(secret), RequireAuth())
	r.GET("/me", func(c *gin.Context) {
		c.JSON(200, gin.H{"uid": UserID(c)})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("want 200, got %d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "user-42") {
		t.Fatalf("body %s", w.Body.String())
	}
}
