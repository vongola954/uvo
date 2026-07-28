package config

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"
)

// IsProduction reports PaaS / public HTTPS deploy.
func (c *Config) IsProduction() bool {
	if strings.HasPrefix(strings.ToLower(c.WebPublicURL), "https://") {
		return true
	}
	if strings.EqualFold(c.DBDriver, "postgres") || strings.EqualFold(c.DBDriver, "postgresql") {
		return true
	}
	if os.Getenv("PORT") != "" && os.Getenv("WEB_PUBLIC_URL") != "" {
		return true
	}
	return false
}

// ApplyProductionGuards forces safe flags and durable secrets for Amvera/prod.
func (c *Config) ApplyProductionGuards() error {
	insecure := os.Getenv("UVO_ALLOW_INSECURE") == "true"

	// Default public URL for known Amvera app if missing (clone/cover/portrait need it).
	if c.WebPublicURL == "" && (strings.EqualFold(c.DBDriver, "postgres") || os.Getenv("PORT") != "") {
		c.WebPublicURL = "https://uvo-baskakovanton.amvera.io"
		_ = os.Setenv("WEB_PUBLIC_URL", c.WebPublicURL)
		logrus.Warn("WEB_PUBLIC_URL was empty — set to https://uvo-baskakovanton.amvera.io")
	}

	dataDir := dataDirFromMedia(c.MediaRoot)
	secret, err := ensureFileSecret(filepath.Join(dataDir, "jwt_secret"), c.JWTSecret, 32)
	if err != nil {
		return fmt.Errorf("jwt secret: %w", err)
	}
	c.JWTSecret = secret
	_ = os.Setenv("JWT_SECRET", secret)

	if wh := os.Getenv("MAX_WEBHOOK_SECRET"); wh == "" && c.BotMode == "webhook" {
		ws, err := ensureFileSecret(filepath.Join(dataDir, "webhook_secret"), "", 24)
		if err != nil {
			return err
		}
		_ = os.Setenv("MAX_WEBHOOK_SECRET", ws)
		logrus.Info("generated MAX_WEBHOOK_SECRET in /data (webhook mode)")
	}

	if c.IsProduction() && !insecure {
		_ = os.Setenv("ALLOW_ANON", "false")
		_ = os.Setenv("DEV_AUTH", "false")
		_ = os.Setenv("DEMO_TOPUP", "false")
		logrus.Info("production guards: ALLOW_ANON/DEV_AUTH/DEMO_TOPUP forced false")
	}

	if c.IsProduction() && len(c.JWTSecret) < 24 {
		return fmt.Errorf("JWT_SECRET too short for production")
	}
	if c.IsProduction() && c.WebPublicURL == "" {
		return fmt.Errorf("WEB_PUBLIC_URL required in production")
	}
	return nil
}

func dataDirFromMedia(mediaRoot string) string {
	if mediaRoot == "" {
		return "./data"
	}
	// /data/media → /data ; ./data/media → ./data
	dir := filepath.Clean(mediaRoot)
	base := filepath.Base(dir)
	if base == "media" {
		return filepath.Dir(dir)
	}
	return dir
}

func ensureFileSecret(path, fromEnv string, nbytes int) (string, error) {
	fromEnv = strings.TrimSpace(fromEnv)
	weak := fromEnv == "" || fromEnv == "dev-secret-change-me" || fromEnv == "change-me-to-long-random-string" || len(fromEnv) < 24
	if !weak {
		return fromEnv, nil
	}
	if b, err := os.ReadFile(path); err == nil {
		s := strings.TrimSpace(string(b))
		if len(s) >= 24 {
			return s, nil
		}
	}
	raw := make([]byte, nbytes)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	s := hex.EncodeToString(raw)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, []byte(s), 0600); err != nil {
		return "", err
	}
	logrus.WithField("path", path).Info("generated durable secret file")
	return s, nil
}
