package middleware

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

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
}
