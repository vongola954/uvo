package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"uvo/internal/models"
)

const lyricsAssistDailyCap = 20

// LyricsAssistDraft asks an OpenAI-compatible chat API for song lyrics (no music credit).
func LyricsAssistDraft(userID, idea, style string, limiter *RateLimiter) (string, error) {
	key := strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
	if key == "" {
		return "", fmt.Errorf("OPENAI_API_KEY не задан")
	}
	idea = strings.TrimSpace(idea)
	if idea == "" {
		return "", fmt.Errorf("нужно описание идеи")
	}
	if utf8.RuneCountInString(idea) > 800 {
		return "", fmt.Errorf("идея максимум 800 символов")
	}
	style = strings.TrimSpace(style)
	if utf8.RuneCountInString(style) > 200 {
		style = string([]rune(style)[:200])
	}

	if limiter != nil && limiter.db != nil {
		dayAgo := time.Now().Add(-24 * time.Hour)
		var n int64
		_ = limiter.db.Model(&models.RateEvent{}).
			Where("user_id = ? AND kind = ? AND created_at > ?", userID, "lyrics_assist", dayAgo).
			Count(&n).Error
		if int(n) >= lyricsAssistDailyCap {
			return "", fmt.Errorf("лимит черновиков текста: %d/сутки", lyricsAssistDailyCap)
		}
		_ = limiter.db.Create(&models.RateEvent{
			UserID: userID, Kind: "lyrics_assist", CreatedAt: time.Now(),
		}).Error
	}

	base := strings.TrimRight(strings.TrimSpace(os.Getenv("OPENAI_BASE_URL")), "/")
	if base == "" {
		base = "https://api.openai.com/v1"
	}
	model := strings.TrimSpace(os.Getenv("OPENAI_MODEL"))
	if model == "" {
		model = "gpt-4o-mini"
	}

	system := "Ты автор песен. Напиши текст песни на русском (куплет/припев), без пояснений, 12–24 строк."
	user := "Идея: " + idea
	if style != "" {
		user += "\nСтиль: " + style
	}

	body := map[string]interface{}{
		"model": model,
		"messages": []map[string]string{
			{"role": "system", "content": system},
			{"role": "user", "content": user},
		},
		"temperature": 0.9,
		"max_tokens":  900,
	}
	raw, _ := json.Marshal(body)
	req, err := http.NewRequest(http.MethodPost, base+"/chat/completions", bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 45 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("llm HTTP %d", resp.StatusCode)
	}
	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return "", err
	}
	if len(out.Choices) == 0 || strings.TrimSpace(out.Choices[0].Message.Content) == "" {
		return "", fmt.Errorf("пустой ответ LLM")
	}
	text := strings.TrimSpace(out.Choices[0].Message.Content)
	if utf8.RuneCountInString(text) > 5000 {
		text = string([]rune(text)[:5000])
	}
	return text, nil
}
