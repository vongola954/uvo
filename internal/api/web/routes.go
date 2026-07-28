package web

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"uvo/internal/api/bot"
	"uvo/internal/clients"
	"uvo/internal/config"
	"uvo/internal/middleware"
	"uvo/internal/repository"
	"uvo/internal/services"
)

type Deps struct {
	Cfg       *config.Config
	DB        *gorm.DB
	Gen       *services.GenerationService
	Credits   *services.CreditService
	Limiter   *services.RateLimiter
	Jobs      *services.JobStore
	Tracks    *repository.TrackRepository
	Voice     *services.VoiceCloneService
	Cover     *services.CoverService
	Karaoke   *services.KaraokeService
	Portrait  *services.PortraitService
	Social    *services.SocialService
	Playlists *services.PlaylistService
	Edit      *services.EditService
	Search    *services.SearchService
	Ace       *clients.AceDataClient
	Eleven    *clients.ElevenLabsClient
	Hedra     *clients.HedraClient
	MaxBot    *bot.Bot
	MaxOn     bool
	Version   string
}

// Register mounts public, webhook and authenticated API groups.
func Register(r *gin.Engine, d *Deps) {
	if d.Version == "" {
		d.Version = "2.6.3"
	}

	r.Static("/static", "./internal/api/web/static")
	r.GET("/", func(c *gin.Context) { c.File("./internal/api/web/static/index.html") })
	r.GET("/tracks.html", func(c *gin.Context) { c.File("./internal/api/web/static/tracks.html") })
	r.GET("/karaoke.html", func(c *gin.Context) { c.File("./internal/api/web/static/karaoke.html") })
	r.GET("/feed.html", func(c *gin.Context) { c.File("./internal/api/web/static/feed.html") })
	r.GET("/playlists.html", func(c *gin.Context) { c.File("./internal/api/web/static/playlists.html") })
	r.GET("/metrics", middleware.MetricsHandler)
	r.GET("/uploads/:name", d.serveUpload)
	r.GET("/media/assets/:name", d.serveMediaAsset)

	r.GET("/health", func(c *gin.Context) {
		aceSt := d.Ace.Status()
		status := "ok"
		if !aceSt.OK {
			status = "degraded"
		}
		hedraOn := d.Hedra != nil && d.Hedra.Enabled()
		c.JSON(200, gin.H{
			"status":         status,
			"version":        d.Version,
			"max_bot":        d.MaxOn,
			"acedata":        aceSt,
			"hedra_portrait": hedraOn,
			"music_provider": "acedata_only",
			"db_driver":      d.Cfg.DBDriver,
			"prod_guards":    d.Cfg != nil && d.Cfg.IsProduction() && os.Getenv("UVO_ALLOW_INSECURE") != "true",
			"allow_anon":     os.Getenv("ALLOW_ANON") == "true",
			"dev_auth":       os.Getenv("DEV_AUTH") == "true",
			"demo_topup":     os.Getenv("DEMO_TOPUP") == "true",
			"web_public_url": d.Cfg.WebPublicURL,
			"hint":           "При provider_balance_empty пополните https://platform.acedata.cloud · вход в веб: MAX /login",
		})
	})

	r.POST("/api/auth/token", d.authToken)
	r.POST("/api/max/webhook", middleware.MaxWebhookAuth(), d.maxWebhook)
	r.GET("/tracks/:id/play", d.playTrack)
	r.GET("/tracks/:id/instrumental", d.playInstrumental)
	r.GET("/tracks/:id/vocals", d.playVocals)
	r.GET("/tracks/:id/video", d.playVideo)
	r.GET("/api/discover", d.discover)

	api := r.Group("/api")
	api.Use(middleware.RequireAuth())
	{
		api.POST("/generate", d.Generate)
		api.GET("/jobs/:id", d.GetJob)
		api.GET("/tracks", d.listTracks)
		api.PATCH("/tracks/:id/visibility", d.setTrackVisibility)
		api.POST("/tracks/:id/karaoke", d.makeKaraoke)
		api.POST("/tracks/:id/portrait", d.makePortrait)
		api.GET("/styles", func(c *gin.Context) { c.JSON(200, gin.H{"styles": services.StyleLibrary}) })
		api.GET("/voices", d.listVoices)
		api.GET("/feed", d.getFeed)
		api.POST("/feed", d.postFeed)
		api.POST("/feed/:id/like", d.likePost)
		api.DELETE("/feed/:id/like", d.unlikePost)
		api.GET("/feed/:id/comments", d.listComments)
		api.POST("/feed/:id/comments", d.addComment)
		api.POST("/playlists", d.createPlaylist)
		api.GET("/playlists", d.listPlaylists)
		api.PATCH("/playlists/:id/visibility", d.setPlaylistVisibility)
		api.POST("/playlists/:id/tracks", d.addPlaylistTrack)
		api.GET("/playlists/:id/tracks", d.getPlaylistTracks)
		api.POST("/bot/simulate", d.botSimulate)
		api.GET("/credits", d.getCredits)
		api.POST("/credits/topup", d.topupCredits)
		api.POST("/tracks/:id/edit", d.editTrack)
		api.GET("/tracks/:id/revisions", d.trackRevisions)
		api.GET("/search", d.search)
		api.DELETE("/tracks/:id", d.deleteTrack)
		api.POST("/voice/clone", d.voiceClone)
		api.POST("/tts", d.tts)
		api.POST("/cover", d.coverUpload)
		api.GET("/elevenlabs/voices", d.elevenVoices)
	}
}

func (d *Deps) authToken(c *gin.Context) {
	if os.Getenv("DEV_AUTH") != "true" {
		middleware.AbortJSON(c, 403, "forbidden", "dev token endpoint disabled (set DEV_AUTH=true)")
		return
	}
	var req struct {
		UserID string `json:"user_id"`
	}
	_ = c.ShouldBindJSON(&req)
	if req.UserID == "" {
		req.UserID = "demo_user"
	}
	tok, err := middleware.IssueToken(d.Cfg.JWTSecret, req.UserID, 24*time.Hour)
	if err != nil {
		middleware.AbortJSON(c, 500, "internal_error", err.Error())
		return
	}
	c.JSON(200, gin.H{"token": tok, "user_id": req.UserID})
}

func (d *Deps) maxWebhook(c *gin.Context) {
	var body struct {
		Updates    []clients.MAXUpdate `json:"updates"`
		UpdateType string              `json:"update_type"`
		ChatID     int64               `json:"chat_id"`
		Message    *clients.MAXMessage `json:"message"`
		User       *clients.MAXUser    `json:"user"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		middleware.AbortJSON(c, 400, "validation_error", err.Error())
		return
	}
	if len(body.Updates) > 0 {
		for _, u := range body.Updates {
			d.MaxBot.HandleWebhookUpdate(u)
		}
	} else if body.UpdateType != "" {
		d.MaxBot.HandleWebhookUpdate(clients.MAXUpdate{
			UpdateType: body.UpdateType, ChatID: body.ChatID, Message: body.Message, User: body.User,
		})
	}
	c.JSON(200, gin.H{"ok": true})
}

func (d *Deps) playTrack(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	track, err := d.Tracks.GetByID(uint(id))
	if err != nil || track == nil {
		middleware.AbortJSON(c, 404, "not_found", "not found")
		return
	}
	uid := middleware.UserID(c)
	if !track.IsPublic && track.UserID != uid {
		if uid == "" {
			middleware.AbortJSON(c, 401, "unauthorized", "Bearer token required")
			return
		}
		middleware.AbortJSON(c, 403, "forbidden", "not your track")
		return
	}
	safe, err := services.SafeMediaPath(d.Cfg.MediaRoot, track.FilePath)
	if err != nil {
		middleware.AbortJSON(c, 403, "forbidden", "forbidden path")
		return
	}
	c.File(safe)
}

func (d *Deps) serveTrackSide(c *gin.Context, which string) {
	id, _ := strconv.Atoi(c.Param("id"))
	track, err := d.Tracks.GetByID(uint(id))
	if err != nil || track == nil {
		middleware.AbortJSON(c, 404, "not_found", "not found")
		return
	}
	uid := middleware.UserID(c)
	if !track.IsPublic && track.UserID != uid {
		if uid == "" {
			middleware.AbortJSON(c, 401, "unauthorized", "Bearer token required")
			return
		}
		middleware.AbortJSON(c, 403, "forbidden", "not your track")
		return
	}
	var path string
	switch which {
	case "instrumental":
		path = track.InstrumentalPath
	case "vocals":
		path = track.VocalsPath
	case "video":
		path = track.VideoPath
	}
	if path == "" {
		middleware.AbortJSON(c, 404, "not_found", which+" not ready — создайте караоке")
		return
	}
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		c.Redirect(302, path)
		return
	}
	safe, err := services.SafeMediaPath(d.Cfg.MediaRoot, path)
	if err != nil {
		middleware.AbortJSON(c, 403, "forbidden", "forbidden path")
		return
	}
	c.File(safe)
}

func (d *Deps) playInstrumental(c *gin.Context) { d.serveTrackSide(c, "instrumental") }
func (d *Deps) playVocals(c *gin.Context)       { d.serveTrackSide(c, "vocals") }
func (d *Deps) playVideo(c *gin.Context)        { d.serveTrackSide(c, "video") }

func (d *Deps) serveMediaAsset(c *gin.Context) {
	name := c.Param("name")
	path, err := services.SafeMediaPath(d.Cfg.MediaRoot, filepath.Join(d.Cfg.MediaRoot, name))
	if err != nil {
		// also try basename only under media root
		path, err = services.SafeMediaPath(d.Cfg.MediaRoot, filepath.Join(d.Cfg.MediaRoot, filepath.Base(name)))
		if err != nil {
			c.Status(404)
			return
		}
	}
	c.File(path)
}

func (d *Deps) listTracks(c *gin.Context) {
	tracks, _ := d.Tracks.GetByUserID(middleware.UserID(c))
	c.JSON(200, gin.H{"tracks": tracks})
}

func (d *Deps) discover(c *gin.Context) {
	tracks, err := d.Tracks.ListPublic(30)
	if err != nil {
		middleware.AbortJSON(c, 500, "internal_error", err.Error())
		return
	}
	c.JSON(200, gin.H{"tracks": tracks})
}

func (d *Deps) setTrackVisibility(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var req struct {
		IsPublic bool `json:"is_public"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.AbortJSON(c, 400, "validation_error", err.Error())
		return
	}
	track, err := d.Tracks.SetPublic(uint(id), middleware.UserID(c), req.IsPublic)
	if err != nil {
		if errors.Is(err, repository.ErrForbidden) {
			middleware.AbortJSON(c, 403, "forbidden", "not your track")
			return
		}
		middleware.AbortJSON(c, 404, "not_found", "not found")
		return
	}
	c.JSON(200, gin.H{"track": track})
}

func (d *Deps) listVoices(c *gin.Context) {
	uid := middleware.UserID(c)
	list, _ := d.Voice.List(uid)
	c.JSON(200, gin.H{"voices": list, "clone_quota_left": d.Voice.QuotaRemaining(uid)})
}

func (d *Deps) getFeed(c *gin.Context) {
	posts, _ := d.Social.Feed(30)
	c.JSON(200, gin.H{"posts": posts})
}

func (d *Deps) postFeed(c *gin.Context) {
	var req struct {
		TrackID uint   `json:"track_id" binding:"required"`
		Caption string `json:"caption"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.AbortJSON(c, 400, "validation_error", err.Error())
		return
	}
	post, err := d.Social.CreatePost(middleware.UserID(c), req.TrackID, req.Caption)
	if err != nil {
		middleware.AbortJSON(c, 403, "forbidden", err.Error())
		return
	}
	c.JSON(200, gin.H{"post": post})
}

func (d *Deps) likePost(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	post, err := d.Social.Like(middleware.UserID(c), uint(id))
	if err != nil {
		middleware.AbortJSON(c, 404, "not_found", err.Error())
		return
	}
	c.JSON(200, gin.H{"post": post})
}

func (d *Deps) unlikePost(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	post, err := d.Social.Unlike(middleware.UserID(c), uint(id))
	if err != nil {
		middleware.AbortJSON(c, 404, "not_found", err.Error())
		return
	}
	c.JSON(200, gin.H{"post": post})
}

func (d *Deps) listComments(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	list, err := d.Social.Comments(uint(id), 50)
	if err != nil {
		middleware.AbortJSON(c, 500, "internal_error", err.Error())
		return
	}
	c.JSON(200, gin.H{"comments": list})
}

func (d *Deps) addComment(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var req struct {
		Text string `json:"text" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.AbortJSON(c, 400, "validation_error", err.Error())
		return
	}
	cm, err := d.Social.AddComment(middleware.UserID(c), uint(id), req.Text)
	if err != nil {
		middleware.AbortJSON(c, 400, "validation_error", err.Error())
		return
	}
	c.JSON(200, gin.H{"comment": cm})
}

func (d *Deps) createPlaylist(c *gin.Context) {
	var req struct {
		Name     string `json:"name" binding:"required"`
		Desc     string `json:"description"`
		IsPublic bool   `json:"is_public"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.AbortJSON(c, 400, "validation_error", err.Error())
		return
	}
	p, err := d.Playlists.Create(middleware.UserID(c), req.Name, req.Desc, req.IsPublic)
	if err != nil {
		middleware.AbortJSON(c, 500, "internal_error", err.Error())
		return
	}
	c.JSON(200, gin.H{"playlist": p})
}

func (d *Deps) setPlaylistVisibility(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var req struct {
		IsPublic bool `json:"is_public"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.AbortJSON(c, 400, "validation_error", err.Error())
		return
	}
	p, err := d.Playlists.SetPublic(middleware.UserID(c), uint(id), req.IsPublic)
	if err != nil {
		if err.Error() == "forbidden" {
			middleware.AbortJSON(c, 403, "forbidden", err.Error())
			return
		}
		middleware.AbortJSON(c, 404, "not_found", err.Error())
		return
	}
	c.JSON(200, gin.H{"playlist": p})
}

func (d *Deps) listPlaylists(c *gin.Context) {
	list, _ := d.Playlists.List(middleware.UserID(c))
	c.JSON(200, gin.H{"playlists": list})
}

func (d *Deps) addPlaylistTrack(c *gin.Context) {
	pid, _ := strconv.Atoi(c.Param("id"))
	var req struct {
		TrackID uint `json:"track_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.AbortJSON(c, 400, "validation_error", err.Error())
		return
	}
	if err := d.Playlists.AddTrack(middleware.UserID(c), uint(pid), req.TrackID); err != nil {
		middleware.AbortJSON(c, 403, "forbidden", err.Error())
		return
	}
	c.JSON(200, gin.H{"ok": true})
}

func (d *Deps) getPlaylistTracks(c *gin.Context) {
	pid, _ := strconv.Atoi(c.Param("id"))
	tracks, err := d.Playlists.GetTracksForUser(middleware.UserID(c), uint(pid))
	if err != nil {
		if err.Error() == "forbidden" {
			middleware.AbortJSON(c, 403, "forbidden", err.Error())
			return
		}
		middleware.AbortJSON(c, 404, "not_found", err.Error())
		return
	}
	c.JSON(200, gin.H{"tracks": tracks})
}

func (d *Deps) botSimulate(c *gin.Context) {
	if os.Getenv("DEV_AUTH") != "true" {
		middleware.AbortJSON(c, 403, "forbidden", "simulate disabled (set DEV_AUTH=true)")
		return
	}
	var req struct {
		UserID string `json:"user_id"`
		Text   string `json:"text" binding:"required"`
		ChatID int64  `json:"chat_id"`
	}
	_ = c.ShouldBindJSON(&req)
	if req.UserID == "" {
		req.UserID = middleware.UserID(c)
	}
	d.MaxBot.HandleText(req.UserID, req.Text, req.ChatID)
	c.JSON(200, gin.H{"ok": true})
}

func (d *Deps) getCredits(c *gin.Context) {
	uid := middleware.UserID(c)
	demo := os.Getenv("DEMO_TOPUP") == "true"
	c.JSON(200, gin.H{
		"balance":    d.Credits.Balance(uid),
		"packs":      services.CreditPacks,
		"demo_topup": demo,
		"payment":    "coming_soon", // real checkout (YooKassa) not wired yet
		"note":       "Оплата картой скоро. Сейчас topup только при DEMO_TOPUP=true (dev).",
	})
}

func (d *Deps) topupCredits(c *gin.Context) {
	if os.Getenv("DEMO_TOPUP") != "true" {
		middleware.AbortJSON(c, 403, "forbidden", "demo topup disabled (set DEMO_TOPUP=true)")
		return
	}
	uid := middleware.UserID(c)
	var req struct {
		PackID  string `json:"pack_id"`
		Credits int    `json:"credits"`
	}
	_ = c.ShouldBindJSON(&req)
	n := req.Credits
	for _, p := range services.CreditPacks {
		if p["id"] == req.PackID {
			n = p["credits"].(int)
			break
		}
	}
	if n <= 0 {
		n = 10
	}
	if n > 1000 {
		n = 1000
	}
	d.Credits.Add(uid, n)
	c.JSON(200, gin.H{"balance": d.Credits.Balance(uid), "added": n, "note": "demo topup without payment"})
}

func (d *Deps) editTrack(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	uid := middleware.UserID(c)
	var req struct {
		Style        string `json:"style"`
		Lyrics       string `json:"lyrics"`
		Prompt       string `json:"prompt"`
		Instrumental bool   `json:"instrumental"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.AbortJSON(c, 400, "validation_error", err.Error())
		return
	}
	if err := d.Credits.Spend(uid, 1); err != nil {
		c.JSON(402, gin.H{"error": gin.H{"code": "insufficient_credits", "message": err.Error()}, "balance": d.Credits.Balance(uid)})
		return
	}
	if err := d.Limiter.Allow(uid); err != nil {
		d.Credits.Refund(uid, 1)
		middleware.AbortJSON(c, 429, "rate_limit", err.Error())
		return
	}
	track, err := d.Edit.Edit(&services.EditRequest{
		UserID: uid, TrackID: uint(id), Style: req.Style, Lyrics: req.Lyrics,
		Prompt: req.Prompt, Instrumental: req.Instrumental,
	})
	if err != nil {
		d.Credits.Refund(uid, 1)
		writeProviderErr(c, err)
		return
	}
	c.JSON(200, gin.H{"track": track, "play_url": fmt.Sprintf("/tracks/%d/play", track.ID)})
}

func (d *Deps) trackRevisions(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	list, err := d.Edit.Revisions(uint(id), middleware.UserID(c))
	if err != nil {
		middleware.AbortJSON(c, 403, "forbidden", err.Error())
		return
	}
	c.JSON(200, gin.H{"revisions": list})
}

func (d *Deps) search(c *gin.Context) {
	tracks, err := d.Search.Tracks(c.Query("q"), middleware.UserID(c), false)
	if err != nil {
		middleware.AbortJSON(c, 500, "internal_error", err.Error())
		return
	}
	c.JSON(200, gin.H{"tracks": tracks})
}

func (d *Deps) deleteTrack(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	uid := middleware.UserID(c)
	track, err := d.Tracks.GetByID(uint(id))
	if err != nil || track == nil || track.UserID != uid {
		middleware.AbortJSON(c, 403, "forbidden", "forbidden")
		return
	}
	if track.FilePath != "" && d.Cfg != nil {
		if safe, err := services.SafeMediaPath(d.Cfg.MediaRoot, track.FilePath); err == nil {
			_ = os.Remove(safe)
		}
	}
	_ = d.DB.Delete(track).Error
	c.JSON(200, gin.H{"ok": true})
}

func (d *Deps) voiceClone(c *gin.Context) {
	uid := middleware.UserID(c)
	const cloneCost = 2
	file, err := c.FormFile("audio")
	if err != nil {
		middleware.AbortJSON(c, 400, "validation_error", "audio file required (multipart field: audio)")
		return
	}
	f, err := file.Open()
	if err != nil {
		middleware.AbortJSON(c, 400, "validation_error", "не удалось открыть файл")
		return
	}
	defer f.Close()
	const maxVoice = 15 << 20
	limited := io.LimitReader(f, maxVoice+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		middleware.AbortJSON(c, 400, "validation_error", "read audio failed")
		return
	}
	if int64(len(data)) > maxVoice {
		middleware.AbortJSON(c, 400, "validation_error", "файл больше 15 MB")
		return
	}
	name := c.PostForm("name")
	if name == "" {
		name = "voice"
	}
	if err := services.ValidateVoiceUpload(name, int64(len(data)), file.Header.Get("Content-Type")); err != nil {
		middleware.AbortJSON(c, 400, "validation_error", err.Error())
		return
	}
	if !services.LooksLikeAudio(data) {
		middleware.AbortJSON(c, 400, "validation_error", "файл не похож на аудио (нужен mp3/wav/m4a/ogg)")
		return
	}
	if err := d.Credits.Spend(uid, cloneCost); err != nil {
		c.JSON(402, gin.H{"error": gin.H{"code": "insufficient_credits", "message": err.Error()}, "balance": d.Credits.Balance(uid)})
		return
	}
	if err := d.Limiter.Allow(uid); err != nil {
		d.Credits.Refund(uid, cloneCost)
		middleware.AbortJSON(c, 429, "rate_limit", err.Error())
		return
	}
	profile, err := d.Voice.Clone(uid, name, data)
	if err != nil {
		d.Credits.Refund(uid, cloneCost)
		if strings.Contains(err.Error(), "квота") {
			middleware.AbortJSON(c, 429, "quota_exceeded", err.Error())
			return
		}
		writeProviderErr(c, err)
		return
	}
	c.JSON(200, gin.H{"voice": profile, "balance": d.Credits.Balance(uid), "cost": cloneCost})
}

func (d *Deps) coverUpload(c *gin.Context) {
	uid := middleware.UserID(c)
	if d.Cover == nil {
		middleware.AbortJSON(c, 501, "not_configured", "cover service unavailable")
		return
	}
	file, err := c.FormFile("audio")
	if err != nil {
		middleware.AbortJSON(c, 400, "validation_error", "audio file required (multipart field: audio) — любой трек для кавера")
		return
	}
	f, err := file.Open()
	if err != nil {
		middleware.AbortJSON(c, 400, "validation_error", err.Error())
		return
	}
	defer f.Close()
	const maxTrack = 30 << 20
	data, err := io.ReadAll(io.LimitReader(f, maxTrack+1))
	if err != nil {
		middleware.AbortJSON(c, 400, "validation_error", "read audio failed")
		return
	}
	if int64(len(data)) > maxTrack {
		middleware.AbortJSON(c, 400, "validation_error", "файл больше 30 MB")
		return
	}
	voiceID := c.PostForm("voice_id")
	if voiceID == "" {
		middleware.AbortJSON(c, 400, "validation_error", "voice_id обязателен — сначала клонируйте голос")
		return
	}
	if err := d.Credits.Spend(uid, 2); err != nil {
		c.JSON(402, gin.H{"error": gin.H{"code": "insufficient_credits", "message": err.Error()}, "balance": d.Credits.Balance(uid)})
		return
	}
	if err := d.Limiter.Allow(uid); err != nil {
		d.Credits.Refund(uid, 2)
		middleware.AbortJSON(c, 429, "rate_limit", err.Error())
		return
	}
	track, err := d.Cover.CoverFromUpload(&services.CoverFromUploadRequest{
		UserID:    uid,
		AudioData: data,
		Filename:  file.Filename,
		VoiceID:   voiceID,
		Prompt:    c.PostForm("prompt"),
		Style:     c.PostForm("style"),
		Lyrics:    c.PostForm("lyrics"),
		Title:     c.PostForm("title"),
	})
	if err != nil {
		d.Credits.Refund(uid, 2)
		writeProviderErr(c, err)
		return
	}
	c.JSON(200, gin.H{
		"success":  true,
		"track":    track,
		"play_url": fmt.Sprintf("/tracks/%d/play", track.ID),
		"balance":  d.Credits.Balance(uid),
		"cost":     2,
	})
}

func (d *Deps) serveUpload(c *gin.Context) {
	if d.Cfg == nil {
		c.Status(404)
		return
	}
	path, err := services.ResolveUploadPath(d.Cfg.MediaRoot, c.Param("name"))
	if err != nil {
		c.Status(404)
		return
	}
	c.File(path)
}

func (d *Deps) makeKaraoke(c *gin.Context) {
	if d.Karaoke == nil {
		middleware.AbortJSON(c, 501, "not_configured", "karaoke unavailable")
		return
	}
	id, _ := strconv.Atoi(c.Param("id"))
	uid := middleware.UserID(c)
	if err := d.Credits.Spend(uid, 2); err != nil {
		c.JSON(402, gin.H{"error": gin.H{"code": "insufficient_credits", "message": err.Error()}, "balance": d.Credits.Balance(uid)})
		return
	}
	if err := d.Limiter.Allow(uid); err != nil {
		d.Credits.Refund(uid, 2)
		middleware.AbortJSON(c, 429, "rate_limit", err.Error())
		return
	}
	res, err := d.Karaoke.Build(uid, uint(id))
	if err != nil {
		d.Credits.Refund(uid, 2)
		writeProviderErr(c, err)
		return
	}
	c.JSON(200, gin.H{"karaoke": res, "balance": d.Credits.Balance(uid), "player_url": fmt.Sprintf("/karaoke.html?id=%d", id)})
}

func (d *Deps) makePortrait(c *gin.Context) {
	if d.Portrait == nil {
		middleware.AbortJSON(c, 501, "not_configured", "portrait unavailable")
		return
	}
	id, _ := strconv.Atoi(c.Param("id"))
	uid := middleware.UserID(c)
	file, err := c.FormFile("image")
	if err != nil {
		middleware.AbortJSON(c, 400, "validation_error", "image required (multipart field: image)")
		return
	}
	f, err := file.Open()
	if err != nil {
		middleware.AbortJSON(c, 400, "validation_error", err.Error())
		return
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, 10<<20+1))
	if err != nil {
		middleware.AbortJSON(c, 400, "validation_error", "read image failed")
		return
	}
	if int64(len(data)) > 10<<20 {
		middleware.AbortJSON(c, 400, "validation_error", "фото больше 10 MB")
		return
	}
	cost := 2
	if d.Hedra != nil && d.Hedra.Enabled() {
		cost = 3
	}
	if err := d.Credits.Spend(uid, cost); err != nil {
		c.JSON(402, gin.H{"error": gin.H{"code": "insufficient_credits", "message": err.Error()}, "balance": d.Credits.Balance(uid)})
		return
	}
	if err := d.Limiter.Allow(uid); err != nil {
		d.Credits.Refund(uid, cost)
		middleware.AbortJSON(c, 429, "rate_limit", err.Error())
		return
	}
	res, err := d.Portrait.Create(uid, uint(id), data, file.Filename, c.PostForm("prompt"))
	if err != nil {
		d.Credits.Refund(uid, cost)
		writeProviderErr(c, err)
		return
	}
	c.JSON(200, gin.H{"portrait": res, "balance": d.Credits.Balance(uid)})
}

func (d *Deps) tts(c *gin.Context) {
	uid := middleware.UserID(c)
	var req struct {
		VoiceID string `json:"voice_id" binding:"required"`
		Text    string `json:"text" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.AbortJSON(c, 400, "validation_error", "voice_id и text обязательны")
		return
	}
	if err := services.ValidateTTS(req.VoiceID, req.Text); err != nil {
		middleware.AbortJSON(c, 400, "validation_error", err.Error())
		return
	}
	if !d.Voice.OwnsVoice(uid, req.VoiceID) {
		middleware.AbortJSON(c, 403, "forbidden", "voice_id does not belong to you")
		return
	}
	const ttsCost = 1
	if err := d.Credits.Spend(uid, ttsCost); err != nil {
		c.JSON(402, gin.H{"error": gin.H{"code": "insufficient_credits", "message": err.Error()}, "balance": d.Credits.Balance(uid)})
		return
	}
	if err := d.Limiter.Allow(uid); err != nil {
		d.Credits.Refund(uid, ttsCost)
		middleware.AbortJSON(c, 429, "rate_limit", err.Error())
		return
	}
	audio, err := d.Voice.TTS(req.VoiceID, req.Text)
	if err != nil {
		d.Credits.Refund(uid, ttsCost)
		writeProviderErr(c, err)
		return
	}
	c.Header("X-Credits-Balance", strconv.Itoa(d.Credits.Balance(uid)))
	c.Data(200, "audio/mpeg", audio)
}

func (d *Deps) elevenVoices(c *gin.Context) {
	if d.Eleven == nil || !d.Eleven.Enabled() {
		middleware.AbortJSON(c, 501, "not_configured", "ELEVENLABS_API_KEY not set")
		return
	}
	list, err := d.Eleven.ListVoices()
	if err != nil {
		writeProviderErr(c, err)
		return
	}
	c.JSON(200, gin.H{"voices": list})
}
