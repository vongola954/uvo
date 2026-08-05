package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
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
	cfg.MediaRoot = services.EnsureMediaRootWritable(cfg.MediaRoot)

	gdb, err := db.Open(cfg)
	if err != nil {
		logrus.Fatal(err)
	}
	config.WarnInsecurePostgresSSL(cfg)

	ace := clients.NewAceDataClient(cfg)
	aceMusic := clients.NewAceMusicClient(cfg)
	sf := clients.NewSiliconFlowClient(cfg.SiliconFlowKey)
	el := clients.NewElevenLabsClient(cfg.ElevenLabsKey)
	hedra := clients.NewHedraClient(os.Getenv("HEDRA_API_KEY"))
	kling := clients.NewKlingClient(cfg.SunoAPIKey)
	maxC := clients.NewMAXClient(cfg.MAXBotToken, cfg.MAXAPIBaseURL)

	trackRepo := repository.NewTrackRepository(gdb)
	userRepo := repository.NewUserRepository(gdb)
	voiceRepo := repository.NewVoiceProfileRepository(gdb)

	genSvc := services.NewGenerationService(ace, aceMusic, cfg.MusicProvider, cfg.MediaRoot, trackRepo, userRepo)
	if aceMusic != nil && aceMusic.Enabled() {
		logrus.Infof("AceMusic enabled (MUSIC_PROVIDER=%s)", cfg.MusicProvider)
	}
	voiceSvc := services.NewVoiceCloneService(ace, sf, el, voiceRepo, userRepo, gdb, cfg.MediaRoot, cfg.WebPublicURL)
	coverSvc := services.NewCoverService(ace, trackRepo, voiceSvc, cfg.MediaRoot, cfg.WebPublicURL)
	karaokeSvc := services.NewKaraokeService(ace, trackRepo, gdb, cfg.MediaRoot, cfg.WebPublicURL)
	portraitSvc := services.NewPortraitService(hedra, kling, trackRepo, gdb, cfg.MediaRoot, cfg.WebPublicURL)
	socialSvc := services.NewSocialService(gdb)
	playlistSvc := services.NewPlaylistService(gdb)
	editSvc := services.NewEditService(ace, trackRepo, gdb, cfg.MediaRoot)
	searchSvc := services.NewSearchService(gdb)
	veo := clients.NewVeoClient()
	upscale := clients.NewUpscaleClient(cfg.SunoAPIKey)
	mediaFX := services.NewMediaFXService(gdb, kling, veo, upscale, cfg.MediaRoot, cfg.WebPublicURL)
	distSvc := services.NewDistributionService(gdb, trackRepo)
	limiter := services.NewRateLimiter(gdb)
	credits := services.NewCreditService(gdb)
	jobs := services.NewJobStore(gdb)
	if n, err := jobs.CleanupOlderThan(7 * 24 * time.Hour); err == nil && n > 0 {
		logrus.Infof("cleaned %d old jobs", n)
	}
	// Stale async jobs → fail + refund; uploads GC.
	go func() {
		t := time.NewTicker(2 * time.Minute)
		defer t.Stop()
		for range t.C {
			if n := jobs.FailStaleAndRefund(services.DefaultStaleJobAge, credits); n > 0 {
				logrus.Warnf("stale jobs refunded: %d", n)
			}
			if n, err := services.CleanupUploadsOlderThan(cfg.MediaRoot, 7*24*time.Hour); err == nil && n > 0 {
				logrus.Infof("cleaned %d old uploads", n)
			}
		}
	}()
	if w := os.Getenv("MAX_WORKERS"); w != "" {
		if n, err := strconv.Atoi(w); err == nil {
			services.SetMaxWorkers(n)
		}
	}

	if cfg.WebPublicURL == "" {
		logrus.Warn("WEB_PUBLIC_URL пуст — клон/кавер/Kling-портрет могут не сработать")
	}
	if hedra == nil || !hedra.Enabled() {
		logrus.Info("HEDRA_API_KEY не задан — портрет через Kling-клип (без точного lip-sync)")
	}
	if mediaFX == nil || !mediaFX.EnabledUpscale() {
		logrus.Info("апскейл: REPLICATE_API_TOKEN (или SUNO_API_KEY для AceData fal)")
	}
	if mediaFX == nil || !mediaFX.EnabledVideo() {
		logrus.Info("видео: VEO_API_KEY/GEMINI_API_KEY или SUNO_API_KEY (Kling)")
	}
	if strings.TrimSpace(os.Getenv("DISTRIBUTION_WEBHOOK_URL")) == "" {
		logrus.Info("DISTRIBUTION_WEBHOOK_URL пуст — релизы в очереди (ручной/партнёрский DSP)")
	}

	logins := services.NewLoginCodeStore()
	yoo := clients.NewYooKassaClient()
	maxBot := bot.New(maxC, genSvc, limiter, credits, logins)
	deps := &webapi.Deps{
		Cfg: cfg, DB: gdb, Gen: genSvc, Credits: credits, Limiter: limiter, Jobs: jobs,
		Tracks: trackRepo, Voice: voiceSvc, Cover: coverSvc, Karaoke: karaokeSvc, Portrait: portraitSvc,
		Social: socialSvc, Playlists: playlistSvc,
		Edit: editSvc, Search: searchSvc, MediaFX: mediaFX, Distribution: distSvc,
		Ace: ace, Eleven: el, Hedra: hedra, Yoo: yoo,
		Logins: logins, MaxBot: maxBot,
		MaxOn: maxC.Enabled(), Version: "2.10.9",
	}
	if cfg.BotMode == "polling" && maxC.Enabled() {
		go maxBot.StartPolling()
	} else if maxC.Enabled() && cfg.BotMode != "webhook" {
		logrus.Warnf("BOT_MODE=%q — polling выключен (ожидается polling|webhook)", cfg.BotMode)
	}
	if cfg.BotMode == "webhook" && os.Getenv("MAX_WEBHOOK_SECRET") == "" {
		logrus.Warn("BOT_MODE=webhook but MAX_WEBHOOK_SECRET is empty — webhook will reject all requests")
	}
	if !maxC.Enabled() {
		logrus.Warn("MAX_BOT_TOKEN пуст — бот не отвечает")
	}
	if cfg.WebPublicURL == "" {
		logrus.Warn("WEB_PUBLIC_URL пуст — задайте в Amvera env (clone/cover/YooKassa return)")
	}

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.MaxMultipartMemory = 32 << 20
	// Amvera / reverse proxies: trust private nets so ClientIP() sees X-Forwarded-For.
	_ = r.SetTrustedProxies([]string{"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "127.0.0.1/32", "::1/128"})
	r.Use(middleware.RecoveryJSON(), gin.Logger(), middleware.Metrics(), middleware.OptionalAuth(cfg.JWTSecret), middleware.CSRF())
	webapi.Register(r, deps)

	if yoo == nil || !yoo.Enabled() {
		logrus.Info("YOOKASSA_* не заданы — checkout недоступен (DEMO_TOPUP для локалки)")
	}
	if services.CreditsUnlimited() {
		logrus.Warn("CREDITS_UNLIMITED=true — кредиты и rate-limit отключены (только для тестов)")
	}
	if services.LyricsAssistEnabled() {
		logrus.Info("lyrics assist enabled (Pollinations free / OpenAI-compatible)")
	}
	logrus.Infof("UVO 2.10.9 on %s:%d (db=%s prod=%v bot=%s max=%v music=%s media=%s)", cfg.WebHost, cfg.WebPort, cfg.DBDriver, cfg.IsProduction(), cfg.BotMode, maxC.Enabled(), cfg.MusicProvider, cfg.MediaRoot)
	_ = r.Run(fmt.Sprintf("%s:%d", cfg.WebHost, cfg.WebPort))
}
