package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"
)

type Config struct {
	MAXBotToken         string
	MAXAPIBaseURL       string
	SunoAPIKey          string `validate:"required"`
	SiliconFlowKey      string
	ElevenLabsKey       string
	OpenAIKey           string
	DBDriver            string // sqlite | postgres
	DBPath              string
	DatabaseURL         string
	MediaRoot           string
	WebHost             string
	WebPort             int `validate:"required"`
	JWTSecret           string
	BotMode             string
	AceDataSunoURL      string
	AceDataTasksURL     string
	AceDataVoicesURL    string
	AceDataUploadURL    string
	AceDataAsync        bool
	AceDataPollInterval int
	AceDataMaxWait      int
	SunoModel           string
	LogPath             string // empty = stdout only (Amvera)
	WebPublicURL        string // public base URL for AceData to fetch uploads
}

func Load() (*Config, error) {
	if err := godotenv.Load(); err != nil {
		// .env is optional for some environments
	}

	cfg := &Config{
		MAXBotToken:         os.Getenv("MAX_BOT_TOKEN"),
		MAXAPIBaseURL:       getEnv("MAX_API_BASE", "https://platform-api2.max.ru"),
		SunoAPIKey:          os.Getenv("SUNO_API_KEY"),
		SiliconFlowKey:      os.Getenv("SILICONFLOW_API_KEY"),
		ElevenLabsKey:       os.Getenv("ELEVENLABS_API_KEY"),
		OpenAIKey:           os.Getenv("OPENAI_API_KEY"),
		DBDriver:            getEnv("DB_DRIVER", "sqlite"),
		DBPath:              getEnv("DB_PATH", "./data/music_bot.db"),
		DatabaseURL:         os.Getenv("DATABASE_URL"),
		MediaRoot:           getEnv("MEDIA_ROOT", "./data/media"),
		WebHost:             getEnv("WEB_HOST", "0.0.0.0"),
		JWTSecret:           getEnv("JWT_SECRET", "dev-secret-change-me"),
		BotMode:             getEnv("BOT_MODE", "polling"),
		AceDataSunoURL:      getEnv("ACEDATA_SUNO_URL", "https://api.acedata.cloud/suno/audios"),
		AceDataTasksURL:     getEnv("ACEDATA_SUNO_TASKS_URL", "https://api.acedata.cloud/suno/tasks"),
		AceDataVoicesURL:    getEnv("ACEDATA_SUNO_VOICES_URL", "https://api.acedata.cloud/suno/voices"),
		AceDataUploadURL:    getEnv("ACEDATA_SUNO_UPLOAD_URL", "https://api.acedata.cloud/suno/upload"),
		SunoModel:           getEnv("SUNO_MODEL", "chirp-v5-5"),
		AceDataAsync:        getEnvBool("ACEDATA_ASYNC", true),
		AceDataPollInterval: getEnvInt("ACEDATA_POLL_INTERVAL", 3),
		AceDataMaxWait:      getEnvInt("ACEDATA_MAX_WAIT", 300),
		LogPath:             os.Getenv("LOG_PATH"),
		WebPublicURL:        strings.TrimRight(os.Getenv("WEB_PUBLIC_URL"), "/"),
	}

	// PORT (PaaS) overrides WEB_PORT
	portStr := getEnv("PORT", getEnv("WEB_PORT", "8010"))
	port, _ := strconv.Atoi(portStr)
	cfg.WebPort = port

	if cfg.DatabaseURL == "" {
		cfg.DatabaseURL = buildPostgresURL()
	}
	if strings.EqualFold(cfg.DBDriver, "postgres") || strings.EqualFold(cfg.DBDriver, "postgresql") {
		if cfg.DatabaseURL == "" {
			return nil, fmt.Errorf("postgres requires DATABASE_URL or PGHOST/PGUSER/PGPASSWORD/PGDATABASE")
		}
	}

	validate := validator.New()
	if err := validate.Struct(cfg); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	if err := cfg.ApplyProductionGuards(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// buildPostgresURL from discrete Amvera-style env vars.
func buildPostgresURL() string {
	host := os.Getenv("PGHOST")
	user := os.Getenv("PGUSER")
	pass := os.Getenv("PGPASSWORD")
	name := os.Getenv("PGDATABASE")
	if host == "" || user == "" || name == "" {
		return ""
	}
	port := getEnv("PGPORT", "5432")
	// Amvera managed Postgres (internal -rw host) expects disable; see deploy/AMVERA.md.
	ssl := getEnv("PGSSLMODE", "disable")
	u := url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(user, pass),
		Host:   net.JoinHostPort(host, port),
		Path:   "/" + name,
	}
	q := u.Query()
	q.Set("sslmode", ssl)
	u.RawQuery = q.Encode()
	return u.String()
}

// WarnInsecurePostgresSSL logs when production uses sslmode=disable (Amvera internal is OK).
func WarnInsecurePostgresSSL(cfg *Config) {
	if cfg == nil || !cfg.IsProduction() {
		return
	}
	if !strings.EqualFold(cfg.DBDriver, "postgres") && !strings.EqualFold(cfg.DBDriver, "postgresql") {
		return
	}
	ssl := strings.ToLower(getEnv("PGSSLMODE", "disable"))
	if ssl == "disable" || ssl == "allow" {
		logrus.Warn("PGSSLMODE=", ssl, " — OK for Amvera internal CNPG; use require for public Postgres hosts")
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if i, err := strconv.Atoi(value); err == nil {
			return i
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	switch os.Getenv(key) {
	case "true", "1":
		return true
	case "false", "0":
		return false
	}
	return defaultValue
}