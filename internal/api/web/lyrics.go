package web

import (
	"os"
	"strings"

	"github.com/gin-gonic/gin"

	"uvo/internal/middleware"
	"uvo/internal/services"
)

func (d *Deps) lyricsAssist(c *gin.Context) {
	uid := middleware.UserID(c)
	var req struct {
		Idea  string `json:"idea"`
		Style string `json:"style"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.AbortJSON(c, 400, "validation_error", err.Error())
		return
	}
	if strings.TrimSpace(os.Getenv("OPENAI_API_KEY")) == "" {
		middleware.AbortJSON(c, 503, "unavailable", "OPENAI_API_KEY не задан — lyrics assist выключен")
		return
	}
	text, err := services.LyricsAssistDraft(uid, req.Idea, req.Style, d.Limiter)
	if err != nil {
		msg := err.Error()
		code := 400
		if strings.Contains(msg, "лимит") {
			code = 429
		} else if strings.Contains(msg, "HTTP") || strings.Contains(msg, "LLM") || strings.Contains(msg, "пустой") {
			code = 502
		}
		middleware.AbortJSON(c, code, "lyrics_assist_error", msg)
		return
	}
	c.JSON(200, gin.H{"lyrics": text, "note": "черновик текста, кредит музыки не списан"})
}
