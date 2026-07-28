package web

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
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
		Prompt       string `json:"prompt" binding:"required"`
		Style        string `json:"style"`
		Lyrics       string `json:"lyrics"`
		Duration     int    `json:"duration"`
		Instrumental bool   `json:"instrumental"`
		VoiceID      string `json:"voice_id"`
		Title        string `json:"title"`
		Sync         bool   `json:"sync"`
		RequestID    string `json:"request_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.AbortJSON(c, 400, "validation_error", err.Error())
		return
	}
	if err := services.ValidateGenerate(req.Prompt, req.Style, req.Lyrics, req.Duration, req.Instrumental); err != nil {
		middleware.AbortJSON(c, 400, "validation_error", err.Error())
		return
	}
	personaID := ""
	if req.VoiceID != "" {
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
		PersonaID: personaID, Title: req.Title,
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
		c.JSON(200, gin.H{
			"success": true, "track_id": track.ID, "title": track.Title,
			"duration": track.Duration, "play_url": fmt.Sprintf("/tracks/%d/play", track.ID),
			"balance": d.Credits.Balance(uid),
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
	if err := d.Limiter.Allow(uid); err != nil {
		d.Credits.Refund(uid, 1)
		d.Jobs.Delete(job.ID)
		middleware.AbortJSON(c, 429, "rate_limit", err.Error())
		return
	}

	services.GoLimited(func() {
		jobID, userID := job.ID, uid
		if !d.Jobs.ClaimProcessing(jobID) {
			// Lost CAS — do not run provider; refund the credit we took as owner.
			d.Credits.Refund(userID, 1)
			return
		}
		track, err := d.Gen.Generate(genReq)
		if err != nil {
			d.Credits.Refund(userID, 1)
			msg := safeErrMessage(err)
			middleware.IncGenFail()
			d.Jobs.Update(jobID, func(j *models.JobRecord) {
				j.Status = string(services.JobFailed)
				j.Error = msg
			})
			return
		}
		middleware.IncGenOK()
		d.Jobs.Update(jobID, func(j *models.JobRecord) {
			j.Status = string(services.JobDone)
			j.TrackID = track.ID
			j.Title = track.Title
			j.Duration = track.Duration
			j.PlayURL = fmt.Sprintf("/tracks/%d/play", track.ID)
		})
	})

	c.JSON(202, gin.H{"job_id": job.ID, "status": job.Status, "poll_url": "/api/jobs/" + job.ID})
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
