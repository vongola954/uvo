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

// Free OpenAI-compatible endpoint (anonymous Pollinations). No signup / key.
// KeylessAI was considered but often fails DNS; Pollinations is their main upstream and works.
const defaultFreeLLMBase = "https://text.pollinations.ai/openai"
const defaultFreeLLMModel = "openai"

type lyricsLLMConfig struct {
	BaseURL string
	APIKey  string
	Model   string
	Provider string // pollinations | openai | custom
	SendAuth bool
}

// LyricsAssistEnabled is true when a real OpenAI key is set or free Pollinations fallback is on.
func LyricsAssistEnabled() bool {
	if strings.EqualFold(strings.TrimSpace(os.Getenv("LYRICS_ASSIST")), "false") {
		return false
	}
	return resolveLyricsLLMConfig().BaseURL != ""
}

func resolveLyricsLLMConfig() lyricsLLMConfig {
	if strings.EqualFold(strings.TrimSpace(os.Getenv("LYRICS_ASSIST")), "false") {
		return lyricsLLMConfig{}
	}
	key := strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
	base := strings.TrimRight(strings.TrimSpace(firstNonEmptyEnv("OPENAI_BASE_URL", "OPENAI_API_BASE")), "/")
	model := strings.TrimSpace(os.Getenv("OPENAI_MODEL"))
	provider := strings.ToLower(strings.TrimSpace(os.Getenv("LYRICS_LLM_PROVIDER")))

	placeholderKey := isPlaceholderAPIKey(key)

	switch provider {
	case "off", "disabled", "none":
		return lyricsLLMConfig{}
	case "pollinations", "free", "keyless":
		if base == "" {
			if provider == "keyless" {
				base = "https://keylessai.thryx.workers.dev/v1"
			} else {
				base = defaultFreeLLMBase
			}
		}
		if model == "" {
			if provider == "keyless" {
				model = "openai-fast"
			} else {
				model = defaultFreeLLMModel
			}
		}
		// Pollinations anonymous must not send a fake Bearer (402). Keyless accepts any string.
		sendAuth := provider == "keyless" || (!placeholderKey && key != "")
		if provider == "keyless" && key == "" {
			key = "not-needed"
			sendAuth = true
		}
		return lyricsLLMConfig{BaseURL: base, APIKey: key, Model: model, Provider: provider, SendAuth: sendAuth}
	case "openai":
		if placeholderKey || key == "" {
			return lyricsLLMConfig{}
		}
		if base == "" {
			base = "https://api.openai.com/v1"
		}
		if model == "" {
			model = "gpt-4o-mini"
		}
		return lyricsLLMConfig{BaseURL: base, APIKey: key, Model: model, Provider: "openai", SendAuth: true}
	}

	// auto: real OpenAI key → OpenAI (or custom base); else free Pollinations
	if key != "" && !placeholderKey {
		if base == "" {
			base = "https://api.openai.com/v1"
		}
		if model == "" {
			model = "gpt-4o-mini"
		}
		prov := "openai"
		if !strings.Contains(base, "api.openai.com") {
			prov = "custom"
		}
		return lyricsLLMConfig{BaseURL: base, APIKey: key, Model: model, Provider: prov, SendAuth: true}
	}

	// Free default — no key required
	if base == "" {
		base = defaultFreeLLMBase
	}
	if model == "" {
		model = defaultFreeLLMModel
	}
	return lyricsLLMConfig{BaseURL: base, APIKey: "", Model: model, Provider: "pollinations", SendAuth: false}
}

func isPlaceholderAPIKey(key string) bool {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "", "not-needed", "none", "free", "keyless", "n/a", "na", "changeme", "your-key", "sk-xxx":
		return true
	default:
		return false
	}
}

func firstNonEmptyEnv(keys ...string) string {
	for _, k := range keys {
		if v := strings.TrimSpace(os.Getenv(k)); v != "" {
			return v
		}
	}
	return ""
}

// LyricsAssistDraft asks an OpenAI-compatible chat API for song lyrics (no music credit).
func LyricsAssistDraft(userID, idea, style string, limiter *RateLimiter) (string, error) {
	cfg := resolveLyricsLLMConfig()
	if cfg.BaseURL == "" {
		return "", fmt.Errorf("lyrics assist выключен (LYRICS_ASSIST=false)")
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

	system := "Ты автор песен. Напиши текст песни на русском (куплет/припев), без пояснений, 12–24 строк."
	user := "Идея: " + idea
	if style != "" {
		user += "\nСтиль: " + style
	}

	body := map[string]interface{}{
		"model": cfg.Model,
		"messages": []map[string]string{
			{"role": "system", "content": system},
			{"role": "user", "content": user},
		},
		"temperature": 0.9,
		"max_tokens":  900,
	}
	raw, _ := json.Marshal(body)
	req, err := http.NewRequest(http.MethodPost, cfg.BaseURL+"/chat/completions", bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	if cfg.SendAuth && cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 300 {
		snip := strings.TrimSpace(string(data))
		if utf8.RuneCountInString(snip) > 180 {
			snip = string([]rune(snip)[:180]) + "…"
		}
		if snip != "" {
			return "", fmt.Errorf("llm HTTP %d: %s", resp.StatusCode, snip)
		}
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
