package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureFileSecretPersists(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "jwt_secret")
	s1, err := ensureFileSecret(path, "dev-secret-change-me", 16)
	if err != nil {
		t.Fatal(err)
	}
	if len(s1) < 24 {
		t.Fatalf("short %s", s1)
	}
	s2, err := ensureFileSecret(path, "weak", 16)
	if err != nil {
		t.Fatal(err)
	}
	if s1 != s2 {
		t.Fatalf("want persist %s != %s", s1, s2)
	}
	strong := "this-is-a-strong-jwt-secret-value-32"
	s3, err := ensureFileSecret(path, strong, 16)
	if err != nil || s3 != strong {
		t.Fatalf("env strong: %s %v", s3, err)
	}
}

func TestApplyProductionGuardsForcesFlags(t *testing.T) {
	t.Setenv("ALLOW_ANON", "true")
	t.Setenv("DEV_AUTH", "true")
	t.Setenv("DEMO_TOPUP", "true")
	t.Setenv("UVO_ALLOW_INSECURE", "")
	cfg := &Config{
		WebPublicURL: "https://uvo-baskakovanton.amvera.io",
		DBDriver:     "postgres",
		JWTSecret:    "dev-secret-change-me",
		MediaRoot:    t.TempDir(),
		BotMode:      "polling",
	}
	if err := cfg.ApplyProductionGuards(); err != nil {
		t.Fatal(err)
	}
	if os.Getenv("ALLOW_ANON") != "false" || os.Getenv("DEV_AUTH") != "false" || os.Getenv("DEMO_TOPUP") != "false" {
		t.Fatalf("flags not forced: anon=%s dev=%s demo=%s", os.Getenv("ALLOW_ANON"), os.Getenv("DEV_AUTH"), os.Getenv("DEMO_TOPUP"))
	}
	if len(cfg.JWTSecret) < 24 {
		t.Fatal("jwt not strengthened")
	}
}

func TestInsecureIgnoredOnPublicHTTPS(t *testing.T) {
	t.Setenv("UVO_ALLOW_INSECURE", "true")
	t.Setenv("ALLOW_ANON", "true")
	t.Setenv("DEMO_TOPUP", "true")
	cfg := &Config{
		WebPublicURL: "https://example.com",
		DBDriver:     "sqlite",
		JWTSecret:    "dev-secret-change-me",
		MediaRoot:    t.TempDir(),
		BotMode:      "polling",
	}
	if insecureEscapeAllowed(cfg.WebPublicURL) {
		t.Fatal("escape must be ignored on https")
	}
	if err := cfg.ApplyProductionGuards(); err != nil {
		t.Fatal(err)
	}
	if !cfg.ProductionGuardsActive() {
		t.Fatal("guards should stay active")
	}
	if os.Getenv("ALLOW_ANON") != "false" {
		t.Fatal("ALLOW_ANON should be forced false despite UVO_ALLOW_INSECURE")
	}
}
