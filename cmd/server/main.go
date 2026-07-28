package main

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"

	"uvo/internal/api/bot"
	webapi "uvo/internal/api/web"
	"uvo/internal/clients"
	"uvo/internal/config"
	"uvo/internal/db"
	"uvo/internal/logger"
	"uvo/internal/middleware"
	"uvo/internal/repository"
	"uvo/internal/services"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		logrus.Fatal(err)
	}
	logger.Init(&logger.Config{Level: "info", OutputPath: cfg.LogPath})
	_ = os.MkdirAll("./data", 0755)
	_ = os.MkdirAll("./logs", 0755)
	_ = os.MkdirAll(cfg.MediaRoot, 0755)

	gdb, err := db.Open(cfg)
	if err != nil {
		logrus.Fatal(err)
	}

	ace := clients.NewAceDataClient(cfg)
	sf := clients.NewSiliconFlowClient(cfg.SiliconFlowKey)
	el := clients.NewElevenLabsClient(cfg.ElevenLabsKey)
	maxC := clients.NewMAXClient(cfg.MAXBotToken, cfg.MAXAPIBaseURL)

	trackRepo := repository.NewTrackRepository(gdb)
	userRepo := repository.NewUserRepository(gdb)
	voiceRepo := repository.NewVoiceProfileRepository(gdb)

	genSvc := services.NewGenerationService(ace, trackRepo, userRepo)
	voiceSvc := services.NewVoiceCloneService(ace, sf, el, voiceRepo, userRepo, gdb, cfg.MediaRoot, cfg.WebPublicURL)
	coverSvc := services.NewCoverService(ace, trackRepo, voiceSvc, cfg.MediaRoot, cfg.WebPublicURL)
	socialSvc := services.NewSocialService(gdb)
	playlistSvc := services.NewPlaylistService(gdb)
	editSvc := services.NewEditService(ace, trackRepo, gdb, cfg.MediaRoot)
	searchSvc := services.NewSearchService(gdb)
	limiter := services.NewRateLimiter(gdb)
	credits := services.NewCreditService(gdb)
	jobs := services.NewJobStore(gdb)
	if n, err := jobs.CleanupOlderThan(7 * 24 * time.Hour); err == nil && n > 0 {
		logrus.Infof("cleaned %d old jobs", n)
	}
	if w := os.Getenv("MAX_WORKERS"); w != "" {
		if n, err := strconv.Atoi(w); err == nil {
			services.SetMaxWorkers(n)
		}
	}

	if cfg.WebPublicURL == "" {
		logrus.Warn("WEB_PUBLIC_URL пуст — клон голоса AceData и каверы из загрузки не сработают (нужен публичный HTTPS)")
	}

	maxBot := bot.New(maxC, genSvc, limiter, credits)
	deps := &webapi.Deps{
		Cfg: cfg, DB: gdb, Gen: genSvc, Credits: credits, Limiter: limiter, Jobs: jobs,
		Tracks: trackRepo, Voice: voiceSvc, Cover: coverSvc, Social: socialSvc, Playlists: playlistSvc,
		Edit: editSvc, Search: searchSvc, Ace: ace, Eleven: el, MaxBot: maxBot,
		MaxOn: maxC.Enabled(), Version: "2.5.0",
	}
	if cfg.BotMode == "polling" && maxC.Enabled() {
		go maxBot.StartPolling()
	}
	if cfg.BotMode == "webhook" && os.Getenv("MAX_WEBHOOK_SECRET") == "" {
		logrus.Warn("BOT_MODE=webhook but MAX_WEBHOOK_SECRET is empty — webhook will reject all requests")
	}

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(middleware.RecoveryJSON(), gin.Logger(), middleware.Metrics(), middleware.OptionalAuth(cfg.JWTSecret), middleware.CSRF())
	webapi.Register(r, deps)

	logrus.Infof("UVO 2.5 on %s:%d (db=%s)", cfg.WebHost, cfg.WebPort, cfg.DBDriver)
	_ = r.Run(fmt.Sprintf("%s:%d", cfg.WebHost, cfg.WebPort))
}
