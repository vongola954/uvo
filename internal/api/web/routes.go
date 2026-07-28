package web

import (
	"fmt"
	"os"
	"strconv"
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
	Social    *services.SocialService
	Playlists *services.PlaylistService
	Edit      *services.EditService
	Search    *services.SearchService
	Ace       *clients.AceDataClient
	Eleven    *clients.ElevenLabsClient
	MaxBot    *bot.Bot
	MaxOn     bool
	Version   string
}

// Register mounts public, webhook and authenticated API groups.
func Register(r *gin.Engine, d *Deps) {
	if d.Version == "" {
		d.Version = "2.2.0"
	}

	r.Static("/static", "./internal/api/web/static")
	r.GET("/", func(c *gin.Context) { c.File("./internal/api/web/static/index.html") })
	r.GET("/tracks.html", func(c *gin.Context) { c.File("./internal/api/web/static/tracks.html") })
	r.GET("/feed.html", func(c *gin.Context) { c.File("./internal/api/web/static/feed.html") })
	r.GET("/playlists.html", func(c *gin.Context) { c.File("./internal/api/web/static/playlists.html") })
	r.GET("/metrics", middleware.MetricsHandler)

	r.GET("/health", func(c *gin.Context) {
		aceSt := d.Ace.Status()
		status := "ok"
		if !aceSt.OK {
			status = "degraded"
		}
		c.JSON(200, gin.H{
			"status":         status,
			"version":        d.Version,
			"max_bot":        d.MaxOn,
			"acedata":        aceSt,
			"music_provider": "acedata_only",
			"db_driver":      d.Cfg.DBDriver,
			"hint":           "При provider_balance_empty пополните https://platform.acedata.cloud",
		})
	})

	r.POST("/api/auth/token", d.authToken)
	r.POST("/api/max/webhook", middleware.MaxWebhookAuth(), d.maxWebhook)
	r.GET("/tracks/:id/play", d.playTrack)

	api := r.Group("/api")
	api.Use(middleware.RequireAuth())
	{
		api.POST("/generate", d.Generate)
		api.GET("/jobs/:id", d.GetJob)
		api.GET("/tracks", d.listTracks)
		api.GET("/styles", func(c *gin.Context) { c.JSON(200, gin.H{"styles": services.StyleLibrary}) })
		api.GET("/voices", d.listVoices)
		api.GET("/feed", d.getFeed)
		api.POST("/feed", d.postFeed)
		api.POST("/playlists", d.createPlaylist)
		api.GET("/playlists", d.listPlaylists)
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

func (d *Deps) listTracks(c *gin.Context) {
	tracks, _ := d.Tracks.GetByUserID(middleware.UserID(c))
	c.JSON(200, gin.H{"tracks": tracks})
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

func (d *Deps) createPlaylist(c *gin.Context) {
	var req struct {
		Name string `json:"name" binding:"required"`
		Desc string `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.AbortJSON(c, 400, "validation_error", err.Error())
		return
	}
	p, err := d.Playlists.Create(middleware.UserID(c), req.Name, req.Desc)
	if err != nil {
		middleware.AbortJSON(c, 500, "internal_error", err.Error())
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
	c.JSON(200, gin.H{"balance": d.Credits.Balance(uid), "packs": services.CreditPacks})
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
	track, err := d.Edit.Edit(&services.EditRequest{
		UserID: uid, TrackID: uint(id), Style: req.Style, Lyrics: req.Lyrics,
		Prompt: req.Prompt, Instrumental: req.Instrumental,
	})
	if err != nil {
		d.Credits.Refund(uid, 1)
		if pe := clients.AsProviderError(err); pe != nil {
			c.JSON(pe.Status, gin.H{"error": gin.H{"code": pe.Code, "message": pe.Message}})
			return
		}
		c.JSON(500, gin.H{"error": gin.H{"code": "generation_failed", "message": err.Error()}})
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
	_ = os.Remove(track.FilePath)
	_ = d.DB.Delete(track).Error
	c.JSON(200, gin.H{"ok": true})
}

func (d *Deps) voiceClone(c *gin.Context) {
	uid := middleware.UserID(c)
	file, err := c.FormFile("audio")
	if err != nil {
		middleware.AbortJSON(c, 400, "validation_error", "audio file required (multipart field: audio)")
		return
	}
	f, err := file.Open()
	if err != nil {
		middleware.AbortJSON(c, 400, "validation_error", err.Error())
		return
	}
	defer f.Close()
	data := make([]byte, file.Size)
	_, _ = f.Read(data)
	name := c.PostForm("name")
	if name == "" {
		name = "voice"
	}
	if err := services.ValidateVoiceUpload(name, file.Size, file.Header.Get("Content-Type")); err != nil {
		middleware.AbortJSON(c, 400, "validation_error", err.Error())
		return
	}
	profile, err := d.Voice.Clone(uid, name, data)
	if err != nil {
		middleware.AbortJSON(c, 500, "internal_error", err.Error())
		return
	}
	c.JSON(200, gin.H{"voice": profile})
}

func (d *Deps) tts(c *gin.Context) {
	uid := middleware.UserID(c)
	var req struct {
		VoiceID string `json:"voice_id" binding:"required"`
		Text    string `json:"text" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.AbortJSON(c, 400, "validation_error", err.Error())
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
	audio, err := d.Voice.TTS(req.VoiceID, req.Text)
	if err != nil {
		middleware.AbortJSON(c, 500, "internal_error", err.Error())
		return
	}
	c.Data(200, "audio/mpeg", audio)
}

func (d *Deps) elevenVoices(c *gin.Context) {
	if d.Eleven == nil || !d.Eleven.Enabled() {
		middleware.AbortJSON(c, 501, "not_configured", "ELEVENLABS_API_KEY not set")
		return
	}
	list, err := d.Eleven.ListVoices()
	if err != nil {
		middleware.AbortJSON(c, 500, "internal_error", err.Error())
		return
	}
	c.JSON(200, gin.H{"voices": list})
}
