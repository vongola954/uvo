package web

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"uvo/internal/middleware"
	"uvo/internal/services"
)

func (d *Deps) upscaleImage(c *gin.Context) {
	if d.MediaFX == nil || !d.MediaFX.EnabledUpscale() {
		middleware.AbortJSON(c, 501, "not_configured", "upscale: задайте REPLICATE_API_TOKEN (или SUNO_API_KEY)")
		return
	}
	uid := middleware.UserID(c)
	data, name, err := readUploadImage(c, "image")
	if err != nil {
		middleware.AbortJSON(c, 400, "validation_error", err.Error())
		return
	}
	if err := d.Credits.Spend(uid, services.UpscaleCost); err != nil {
		c.JSON(402, gin.H{"error": gin.H{"code": "insufficient_credits", "message": err.Error()}, "balance": d.Credits.Balance(uid)})
		return
	}
	if err := d.Limiter.Allow(uid); err != nil {
		d.Credits.Refund(uid, services.UpscaleCost)
		middleware.AbortJSON(c, 429, "rate_limit", err.Error())
		return
	}
	res, err := d.MediaFX.Upscale(uid, data, name)
	if err != nil {
		d.Credits.Refund(uid, services.UpscaleCost)
		writeProviderErr(c, err)
		return
	}
	c.JSON(200, gin.H{"result": res, "balance": d.Credits.Balance(uid)})
}

func (d *Deps) animateImage(c *gin.Context) {
	if d.MediaFX == nil || !d.MediaFX.EnabledAnimate() {
		middleware.AbortJSON(c, 501, "not_configured", "animate: нужен SUNO_API_KEY (Kling)")
		return
	}
	uid := middleware.UserID(c)
	data, name, err := readUploadImage(c, "image")
	if err != nil {
		middleware.AbortJSON(c, 400, "validation_error", err.Error())
		return
	}
	if err := d.Credits.Spend(uid, services.AnimateCost); err != nil {
		c.JSON(402, gin.H{"error": gin.H{"code": "insufficient_credits", "message": err.Error()}, "balance": d.Credits.Balance(uid)})
		return
	}
	if err := d.Limiter.Allow(uid); err != nil {
		d.Credits.Refund(uid, services.AnimateCost)
		middleware.AbortJSON(c, 429, "rate_limit", err.Error())
		return
	}
	res, err := d.MediaFX.Animate(uid, data, name, c.PostForm("prompt"))
	if err != nil {
		d.Credits.Refund(uid, services.AnimateCost)
		writeProviderErr(c, err)
		return
	}
	c.JSON(200, gin.H{"result": res, "balance": d.Credits.Balance(uid)})
}

func (d *Deps) generateVideo(c *gin.Context) {
	if d.MediaFX == nil || !d.MediaFX.EnabledVideo() {
		middleware.AbortJSON(c, 501, "not_configured", "video: VEO_API_KEY/GEMINI_API_KEY или SUNO_API_KEY")
		return
	}
	uid := middleware.UserID(c)
	prompt := strings.TrimSpace(c.PostForm("prompt"))
	if prompt == "" {
		var req struct {
			Prompt string `json:"prompt"`
		}
		_ = c.ShouldBindJSON(&req)
		prompt = strings.TrimSpace(req.Prompt)
	}
	var img []byte
	var fname string
	if f, err := c.FormFile("image"); err == nil && f != nil {
		img, fname, err = readUploadImage(c, "image")
		if err != nil {
			middleware.AbortJSON(c, 400, "validation_error", err.Error())
			return
		}
	}
	if err := d.Credits.Spend(uid, services.VideoCost); err != nil {
		c.JSON(402, gin.H{"error": gin.H{"code": "insufficient_credits", "message": err.Error()}, "balance": d.Credits.Balance(uid)})
		return
	}
	if err := d.Limiter.Allow(uid); err != nil {
		d.Credits.Refund(uid, services.VideoCost)
		middleware.AbortJSON(c, 429, "rate_limit", err.Error())
		return
	}
	res, err := d.MediaFX.GenerateVideo(uid, prompt, img, fname)
	if err != nil {
		d.Credits.Refund(uid, services.VideoCost)
		writeProviderErr(c, err)
		return
	}
	c.JSON(200, gin.H{"result": res, "balance": d.Credits.Balance(uid)})
}

func (d *Deps) listMediaFX(c *gin.Context) {
	if d.MediaFX == nil {
		c.JSON(200, gin.H{"assets": []any{}})
		return
	}
	list, err := d.MediaFX.List(middleware.UserID(c), c.Query("kind"))
	if err != nil {
		middleware.AbortJSON(c, 500, "internal_error", err.Error())
		return
	}
	c.JSON(200, gin.H{"assets": list})
}

func (d *Deps) distributeTrack(c *gin.Context) {
	if d.Distribution == nil {
		middleware.AbortJSON(c, 501, "not_configured", "distribution unavailable")
		return
	}
	uid := middleware.UserID(c)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		middleware.AbortJSON(c, 400, "validation_error", "invalid track id")
		return
	}
	var req struct {
		Title     string   `json:"title"`
		Artist    string   `json:"artist"`
		Genre     string   `json:"genre"`
		Platforms []string `json:"platforms"`
		Notes     string   `json:"notes"`
	}
	_ = c.ShouldBindJSON(&req)
	if existing, _ := d.Distribution.FindActive(uid, uint(id)); existing != nil {
		c.JSON(200, gin.H{
			"release":    existing,
			"balance":    d.Credits.Balance(uid),
			"idempotent": true,
			"note":       "Релиз уже в очереди / на площадках — повторная отправка не списала кредиты.",
		})
		return
	}
	if err := d.Credits.Spend(uid, services.DistributionCost); err != nil {
		c.JSON(402, gin.H{"error": gin.H{"code": "insufficient_credits", "message": err.Error()}, "balance": d.Credits.Balance(uid)})
		return
	}
	rel, err := d.Distribution.Submit(&services.DistributeRequest{
		UserID: uid, TrackID: uint(id), Title: req.Title, Artist: req.Artist, Genre: req.Genre, Platforms: req.Platforms, Notes: req.Notes,
	})
	if err != nil {
		d.Credits.Refund(uid, services.DistributionCost)
		code := 400
		if strings.Contains(err.Error(), "unavailable") {
			code = 503
		} else if !strings.Contains(err.Error(), "not found") && !strings.Contains(err.Error(), "already") {
			code = 502
		}
		middleware.AbortJSON(c, code, "distribution_error", err.Error())
		return
	}
	c.JSON(200, gin.H{
		"release": rel,
		"balance": d.Credits.Balance(uid),
		"note":    "Релиз в очереди на Spotify / Яндекс Музыку / Apple / VK.",
	})
}

func (d *Deps) listDistribution(c *gin.Context) {
	if d.Distribution == nil {
		c.JSON(200, gin.H{"releases": []any{}})
		return
	}
	list, err := d.Distribution.List(middleware.UserID(c))
	if err != nil {
		middleware.AbortJSON(c, 500, "internal_error", err.Error())
		return
	}
	c.JSON(200, gin.H{"releases": list})
}

func (d *Deps) distributionWebhook(c *gin.Context) {
	secret := strings.TrimSpace(c.GetHeader("X-UVO-Distribution-Secret"))
	want := strings.TrimSpace(os.Getenv("DISTRIBUTION_WEBHOOK_SECRET"))
	if want == "" {
		middleware.AbortJSON(c, 503, "not_configured", "DISTRIBUTION_WEBHOOK_SECRET required")
		return
	}
	if !middleware.SecretEqual(secret, want) {
		middleware.AbortJSON(c, 401, "unauthorized", "bad secret")
		return
	}
	var req struct {
		ExternalID string `json:"external_id"`
		Status     string `json:"status"`
		Note       string `json:"note"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.ExternalID == "" {
		middleware.AbortJSON(c, 400, "validation_error", "external_id required")
		return
	}
	if d.Distribution == nil {
		middleware.AbortJSON(c, 501, "not_configured", "distribution unavailable")
		return
	}
	if err := d.Distribution.ApplyWebhookUpdate(req.ExternalID, req.Status, req.Note); err != nil {
		middleware.AbortJSON(c, 404, "not_found", err.Error())
		return
	}
	c.JSON(200, gin.H{"ok": true})
}

func readUploadImage(c *gin.Context, field string) ([]byte, string, error) {
	file, err := c.FormFile(field)
	if err != nil {
		return nil, "", err
	}
	f, err := file.Open()
	if err != nil {
		return nil, "", err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, 10<<20+1))
	if err != nil {
		return nil, "", err
	}
	if len(data) > 10<<20 {
		return nil, "", errors.New("фото больше 10 MB")
	}
	name := file.Filename
	if name == "" {
		name = "image.jpg"
	}
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".jpg", ".jpeg", ".png", ".webp", ".gif":
	default:
		name = name + ".jpg"
	}
	return data, name, nil
}
