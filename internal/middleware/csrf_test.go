package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestCSRFGenerateRequiresHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(CSRF())
	r.POST("/api/generate", func(c *gin.Context) { c.Status(200) })
	r.GET("/warm", func(c *gin.Context) { c.Status(200) })

	warm := httptest.NewRecorder()
	reqWarm := httptest.NewRequest(http.MethodGet, "/warm", nil)
	r.ServeHTTP(warm, reqWarm)
	cookie := warm.Result().Cookies()
	var csrf string
	for _, c := range cookie {
		if c.Name == "uvo_csrf" {
			csrf = c.Value
		}
	}
	if csrf == "" {
		t.Fatal("expected csrf cookie")
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/generate", nil)
	req.AddCookie(&http.Cookie{Name: "uvo_csrf", Value: csrf})
	r.ServeHTTP(w, req)
	if w.Code != 403 {
		t.Fatalf("no header want 403, got %d", w.Code)
	}

	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodPost, "/api/generate", nil)
	req2.AddCookie(&http.Cookie{Name: "uvo_csrf", Value: csrf})
	req2.Header.Set("X-CSRF-Token", csrf)
	r.ServeHTTP(w2, req2)
	if w2.Code != 200 {
		t.Fatalf("with header want 200, got %d", w2.Code)
	}
}

func TestCSRFAuthExchangeRequiresHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(CSRF())
	r.POST("/api/auth/exchange", func(c *gin.Context) { c.Status(200) })
	r.GET("/warm", func(c *gin.Context) { c.Status(200) })

	warm := httptest.NewRecorder()
	r.ServeHTTP(warm, httptest.NewRequest(http.MethodGet, "/warm", nil))
	var csrf string
	for _, c := range warm.Result().Cookies() {
		if c.Name == "uvo_csrf" {
			csrf = c.Value
		}
	}
	if csrf == "" {
		t.Fatal("csrf cookie")
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/exchange", nil)
	req.AddCookie(&http.Cookie{Name: "uvo_csrf", Value: csrf})
	r.ServeHTTP(w, req)
	if w.Code != 403 {
		t.Fatalf("exchange without CSRF want 403, got %d", w.Code)
	}

	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodPost, "/api/auth/exchange", nil)
	req2.AddCookie(&http.Cookie{Name: "uvo_csrf", Value: csrf})
	req2.Header.Set("X-CSRF-Token", csrf)
	r.ServeHTTP(w2, req2)
	if w2.Code != 200 {
		t.Fatalf("exchange with CSRF want 200, got %d", w2.Code)
	}
}

func TestCSRFMaxWebAppExempt(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(CSRF())
	r.POST("/api/auth/max-webapp", func(c *gin.Context) { c.Status(200) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/max-webapp", nil)
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("max-webapp should skip CSRF, got %d", w.Code)
	}
}

func TestCSRFDistributionWebhookExempt(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(CSRF())
	r.POST("/api/distribution/webhook", func(c *gin.Context) { c.Status(200) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/distribution/webhook", nil)
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("distribution webhook should skip CSRF, got %d", w.Code)
	}
}
