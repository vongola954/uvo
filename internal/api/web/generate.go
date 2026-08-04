package web

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"uvo/internal/clients"
	"uvo/internal/middleware"
	"uvo/internal/models"
	"uvo/internal/services"
)

func (d *Deps) Generate(c *gin.Context) {
	uid := middleware.UserID(c)
	if uid == "" {
		middleware.AbortJSON(c, 401, "unauthorized", "Bearer token required (or ALLOW_ANON=true for demo)")
		return
	}
	var req struct {
		Mode         string `json:"mode"` // idea | lyrics | instrumental
		Prompt       string `json:"prompt"`
		Style        string `json:"style"`
		Lyrics       string `json:"lyrics"`
		Duration     int    `json:"duration"`
		Instrumental bool   `json:"instrumental"`
		VoiceID      string `json:"voice_id"`
		Title        string `json:"title"`
		Sync         bool   `json:"sync"`
		RequestID    string `json:"request_id"`
		Provider     string `json:"provider"` // auto | acedata | acemusic
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.AbortJSON(c, 400, "validation_error", err.Error())
		return
	}
	mode := services.NormalizeGenerateMode(req.Mode, req.Instrumental)
	if mode == "instrumental" {
		req.Instrumental = true
	}
	if err := services.ValidateGenerateMode(mode, req.Prompt, req.Style, req.Lyrics, req.Duration, req.Instrumental); err != nil {
		middleware.AbortJSON(c, 400, "validation_error", err.Error())
		return
	}
	provider := strings.ToLower(strings.TrimSpace(req.Provider))
	if provider == "" {
		provider = "auto"
	}
	personaID := ""
	if req.VoiceID != "" && provider != "acemusic" {
		if d.Voice == nil {
			middleware.AbortJSON(c, 400, "validation_error", "voice service unavailable")
			return
		}
		personaID = d.Voice.AcePersonaID(uid, req.VoiceID)
		if personaID == "" {
			middleware.AbortJSON(c, 400, "validation_error", "voice_id должен быть AceData-клоном (для Suno). Сначала клонируйте голос в студии.")
			return
		}
	}

	// Idempotency: existing job → no spend
	if req.RequestID != "" && !req.Sync {
		if j, ok := d.Jobs.GetByRequestID(uid, req.RequestID); ok {
			c.JSON(http.StatusAccepted, gin.H{"job_id": j.ID, "status": j.Status, "poll_url": "/api/jobs/" + j.ID, "idempotent": true})
			return
		}
	}

	req.Duration = services.ClampDuration(req.Duration, 480)
	genReq := &services.GenerateRequest{
		UserID: uid, Prompt: req.Prompt, Style: req.Style, Lyrics: req.Lyrics,
		Duration: req.Duration, Instrumental: req.Instrumental, VoiceID: req.VoiceID,
		PersonaID: personaID, Title: req.Title, Provider: provider,
	}

	if req.Sync {
		if err := d.Credits.Spend(uid, 1); err != nil {
			c.JSON(402, gin.H{"error": gin.H{"code": "insufficient_credits", "message": err.Error()}, "balance": d.Credits.Balance(uid)})
			return
		}
		if err := d.Limiter.Allow(uid); err != nil {
			d.Credits.Refund(uid, 1)
			middleware.AbortJSON(c, 429, "rate_limit", err.Error())
			return
		}
		track, err := d.Gen.Generate(genReq)
		if err != nil {
			d.Credits.Refund(uid, 1)
			middleware.IncGenFail()
			writeProviderErr(c, err)
			return
		}
		middleware.IncGenOK()
		play := fmt.Sprintf("/tracks/%d/play", track.ID)
		secret := ""
		if d.Cfg != nil {
			secret = d.Cfg.JWTSecret
		}
		dl := services.SignTrackDownloadURL(track.ID, secret)
		c.JSON(200, gin.H{
			"success": true, "track_id": track.ID, "title": track.Title,
			"duration": track.Duration, "play_url": play, "download_url": dl,
			"mode": mode, "balance": d.Credits.Balance(uid),
		})
		return
	}

	// Async: claim job first, then spend — only the create winner pays / starts worker.
	job, created := d.Jobs.CreateOrClaim(uid, req.RequestID)
	if !created {
		c.JSON(http.StatusAccepted, gin.H{
			"job_id": job.ID, "status": job.Status, "poll_url": "/api/jobs/" + job.ID, "idempotent": true,
		})
		return
	}

	if err := d.Credits.Spend(uid, 1); err != nil {
		d.Jobs.Delete(job.ID)
		c.JSON(402, gin.H{"error": gin.H{"code": "insufficient_credits", "message": err.Error()}, "balance": d.Credits.Balance(uid)})
		return
	}
	d.Jobs.SetCreditsSpent(job.ID, 1)
	if err := d.Limiter.Allow(uid); err != nil {
		d.Credits.Refund(uid, 1)
		d.Jobs.Delete(job.ID)
		middleware.AbortJSON(c, 429, "rate_limit", err.Error())
		return
	}

	services.GoLimited(func() {
		jobID, userID := job.ID, uid
		secret := ""
		if d.Cfg != nil {
			secret = d.Cfg.JWTSecret
		}
		if !d.Jobs.ClaimProcessing(jobID) {
			// Lost CAS — do not run provider; refund the credit we took as owner.
			d.Credits.Refund(userID, 1)
			d.Jobs.MarkFailedRefunded(jobID, "lost claim")
			return
		}
		tracks, err := d.Gen.GenerateAll(genReq)
		if err != nil {
			d.Credits.Refund(userID, 1)
			msg := safeErrMessage(err)
			middleware.IncGenFail()
			logrus.WithError(err).WithFields(logrus.Fields{
				"job_id":  jobID,
				"user_id": userID,
			}).Warn("generate job failed")
			d.Jobs.Update(jobID, func(j *models.JobRecord) {
				j.Status = string(services.JobFailed)
				j.Error = msg
				j.Refunded = true
			})
			return
		}
		track := tracks[0]
		middleware.IncGenOK()
		d.Jobs.Update(jobID, func(j *models.JobRecord) {
			j.Status = string(services.JobDone)
			j.TrackID = track.ID
			j.Title = track.Title
			j.Duration = track.Duration
			j.PlayURL = fmt.Sprintf("/tracks/%d/play", track.ID)
			j.DownloadURL = services.SignTrackDownloadURL(track.ID, secret)
			if len(tracks) > 1 {
				j.AltTrackID = tracks[1].ID
				j.AltPlayURL = fmt.Sprintf("/tracks/%d/play", tracks[1].ID)
			}
		})
	})

	c.JSON(202, gin.H{"job_id": job.ID, "status": job.Status, "poll_url": "/api/jobs/" + job.ID, "mode": mode})
}

func (d *Deps) GetJob(c *gin.Context) {
	j, ok := d.Jobs.Get(c.Param("id"))
	if !ok {
		c.JSON(404, gin.H{"error": "not found"})
		return
	}
	uid := middleware.UserID(c)
	if j.UserID != uid {
		c.JSON(404, gin.H{"error": "not found"})
		return
	}
	if j.Status == string(services.JobDone) && j.TrackID > 0 && d.Cfg != nil {
		j.DownloadURL = services.SignTrackDownloadURL(j.TrackID, d.Cfg.JWTSecret)
	}
	c.JSON(200, j)
}

func writeProviderErr(c *gin.Context, err error) {
	if pe := clients.AsProviderError(err); pe != nil {
		c.JSON(pe.Status, gin.H{"error": gin.H{"code": pe.Code, "message": pe.Message}})
		return
	}
	c.JSON(500, gin.H{"error": gin.H{"code": "provider_error", "message": "Операция не удалась. Попробуйте позже."}})
}

func safeErrMessage(err error) string {
	if pe := clients.AsProviderError(err); pe != nil {
		return pe.Message
	}
	return "Операция не удалась. Попробуйте позже."
}
