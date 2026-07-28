package web_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	webapi "uvo/internal/api/web"
	"uvo/internal/clients"
	"uvo/internal/config"
	"uvo/internal/middleware"
	"uvo/internal/models"
	"uvo/internal/services"
)

func TestTopupRequiresDemoFlag(t *testing.T) {
	gin.SetMode(gin.TestMode)
	os.Unsetenv("DEMO_TOPUP")
	os.Setenv("ALLOW_ANON", "true")
	t.Cleanup(func() { os.Unsetenv("ALLOW_ANON") })

	db, err := gorm.Open(sqlite.Open("file:topup_e11?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.CreditBalance{}); err != nil {
		t.Fatal(err)
	}
	deps := &webapi.Deps{
		Cfg:     &config.Config{DBDriver: "sqlite", MediaRoot: t.TempDir(), JWTSecret: "t"},
		Credits: services.NewCreditService(db),
		Ace:     clients.NewAceDataClient(&config.Config{}),
		Version: "test",
	}

	r := gin.New()
	r.Use(middleware.OptionalAuth("t"))
	webapi.Register(r, deps)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/credits/topup", nil)
	r.ServeHTTP(w, req)
	if w.Code != 403 {
		t.Fatalf("want 403 without DEMO_TOPUP, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestGenerateRequiresAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	os.Unsetenv("ALLOW_ANON")
	os.Unsetenv("DEMO_TOPUP")

	deps := &webapi.Deps{
		Cfg:     &config.Config{DBDriver: "sqlite", MediaRoot: t.TempDir(), JWTSecret: "t"},
		Ace:     clients.NewAceDataClient(&config.Config{}),
		Version: "test",
	}
	r := gin.New()
	r.Use(middleware.OptionalAuth("t"))
	webapi.Register(r, deps)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/generate", nil)
	r.ServeHTTP(w, req)
	if w.Code != 401 {
		t.Fatalf("want 401, got %d body=%s", w.Code, w.Body.String())
	}
}
