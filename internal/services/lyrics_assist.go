package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"gorm.io/gorm"

	"uvo/internal/models"
)

const lyricsAssistDailyCap = 20

// Free OpenAI-compatible endpoint (anonymous Pollinations). Often returns 402 from
// datacenter IPs / after deprecation — localDraft is the guaranteed fallback.
const defaultFreeLLMBase = "https://text.pollinations.ai/openai"
const defaultFreeLLMModel = "openai"

type lyricsLLMConfig struct {
	BaseURL  string
	APIKey   string
	Model    string
	Provider string // pollinations | openai | custom | keyless | gemini | local
	SendAuth bool
}

// LyricsAssistEnabled is true unless explicitly disabled.
func LyricsAssistEnabled() bool {
	if strings.EqualFold(strings.TrimSpace(os.Getenv("LYRICS_ASSIST")), "false") {
		return false
	}
	return true
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
	case "local":
		return lyricsLLMConfig{Provider: "local"}
	case "pollinations", "free":
		return freePollinationsConfig(base, model)
	case "keyless":
		if base == "" {
			base = "https://keylessai.thryx.workers.dev/v1"
		}
		if model == "" {
			model = "openai-fast"
		}
		// Never forward a real OPENAI_API_KEY to a third-party keyless proxy.
		return lyricsLLMConfig{BaseURL: base, APIKey: "not-needed", Model: model, Provider: "keyless", SendAuth: true}
	case "gemini":
		gkey := firstNonEmptyEnv("GEMINI_API_KEY", "VEO_API_KEY", "GOOGLE_API_KEY")
		if gkey == "" {
			return lyricsLLMConfig{Provider: "local"}
		}
		if model == "" {
			model = "gemini-2.0-flash"
		}
		return lyricsLLMConfig{APIKey: gkey, Model: model, Provider: "gemini", SendAuth: true}
	case "openai":
		if placeholderKey || key == "" {
			return lyricsLLMConfig{Provider: "local"}
		}
		if base == "" {
			base = "https://api.openai.com/v1"
		}
		if model == "" {
			model = "gpt-4o-mini"
		}
		return finalizeLLMAuth(lyricsLLMConfig{BaseURL: base, APIKey: key, Model: model, Provider: "openai", SendAuth: true})
	}

	// auto: gemini key → gemini; real OpenAI key (non-pollinations base) → openai; else pollinations→local
	if gkey := firstNonEmptyEnv("GEMINI_API_KEY", "VEO_API_KEY", "GOOGLE_API_KEY"); gkey != "" && (provider == "" || provider == "auto") {
		m := model
		if m == "" || strings.HasPrefix(m, "gpt-") || m == "openai" {
			m = "gemini-2.0-flash"
		}
		return lyricsLLMConfig{APIKey: gkey, Model: m, Provider: "gemini", SendAuth: true}
	}

	if isPollinationsBase(base) {
		return freePollinationsConfig(base, model)
	}

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
		return finalizeLLMAuth(lyricsLLMConfig{BaseURL: base, APIKey: key, Model: model, Provider: prov, SendAuth: true})
	}

	return freePollinationsConfig(base, model)
}

func freePollinationsConfig(base, model string) lyricsLLMConfig {
	if base == "" || !isPollinationsBase(base) {
		base = defaultFreeLLMBase
	}
	if model == "" || model == "gpt-4o-mini" || model == "gpt-4o" {
		model = defaultFreeLLMModel
	}
	return lyricsLLMConfig{
		BaseURL:  strings.TrimRight(base, "/"),
		APIKey:   "",
		Model:    model,
		Provider: "pollinations",
		SendAuth: false,
	}
}

func finalizeLLMAuth(cfg lyricsLLMConfig) lyricsLLMConfig {
	if isPollinationsBase(cfg.BaseURL) {
		cfg.APIKey = ""
		cfg.SendAuth = false
		cfg.Provider = "pollinations"
		if cfg.Model == "" || cfg.Model == "gpt-4o-mini" || cfg.Model == "gpt-4o" {
			cfg.Model = defaultFreeLLMModel
		}
	}
	return cfg
}

func isPollinationsBase(base string) bool {
	base = strings.ToLower(strings.TrimSpace(base))
	if base == "" {
		return false
	}
	if strings.Contains(base, "pollinations.ai") {
		return true
	}
	u, err := url.Parse(base)
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	return strings.HasSuffix(host, "pollinations.ai")
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
	if !LyricsAssistEnabled() {
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
		err := limiter.db.Transaction(func(tx *gorm.DB) error {
			var n int64
			if err := tx.Model(&models.RateEvent{}).
				Where("user_id = ? AND kind = ? AND created_at > ?", userID, "lyrics_assist", dayAgo).
				Count(&n).Error; err != nil {
				return err
			}
			if int(n) >= lyricsAssistDailyCap {
				return fmt.Errorf("лимит черновиков текста: %d/сутки", lyricsAssistDailyCap)
			}
			return tx.Create(&models.RateEvent{
				UserID: userID, Kind: "lyrics_assist", CreatedAt: time.Now(),
			}).Error
		})
		if err != nil {
			return "", err
		}
	}

	if cfg.Provider == "local" {
		return localLyricsDraft(idea, style), nil
	}

	system := "Ты автор песен. Напиши текст песни на русском (куплет/припев), без пояснений, 12–24 строк."
	user := "Идея: " + idea
	if style != "" {
		user += "\nСтиль: " + style
	}

	var text string
	var err error
	switch cfg.Provider {
	case "gemini":
		text, err = callGemini(cfg, system, user)
	default:
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
		text, err = callChatCompletions(cfg, raw)
		// Pollinations often 402 from cloud IPs — retry once with browser-like headers already in call;
		// then fall back to Gemini if key exists; else local draft.
		if err != nil && (strings.Contains(err.Error(), "402") || strings.Contains(err.Error(), "401") || strings.Contains(err.Error(), "429")) {
			if gkey := firstNonEmptyEnv("GEMINI_API_KEY", "VEO_API_KEY", "GOOGLE_API_KEY"); gkey != "" {
				text, err = callGemini(lyricsLLMConfig{APIKey: gkey, Model: "gemini-2.0-flash", Provider: "gemini"}, system, user)
			}
		}
	}
	if err != nil || strings.TrimSpace(text) == "" {
		// Guaranteed draft so the Sonata-like wizard never blocks on free LLM outages.
		return localLyricsDraft(idea, style), nil
	}
	if utf8.RuneCountInString(text) > 5000 {
		text = string([]rune(text)[:5000])
	}
	return text, nil
}

func callChatCompletions(cfg lyricsLLMConfig, raw []byte) (string, error) {
	if strings.TrimSpace(cfg.BaseURL) == "" {
		return "", fmt.Errorf("llm base empty")
	}
	req, err := http.NewRequest(http.MethodPost, cfg.BaseURL+"/chat/completions", bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	// Never send Authorization to Pollinations — Bearer forces paid mode → 402.
	if cfg.SendAuth && cfg.APIKey != "" && !isPollinationsBase(cfg.BaseURL) {
		req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if isPollinationsBase(cfg.BaseURL) {
		req.Header.Set("Referer", "https://pollinations.ai/")
		req.Header.Set("Origin", "https://pollinations.ai")
		req.Header.Del("Authorization")
	}
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
	return strings.TrimSpace(out.Choices[0].Message.Content), nil
}

func callGemini(cfg lyricsLLMConfig, system, user string) (string, error) {
	if strings.TrimSpace(cfg.APIKey) == "" {
		return "", fmt.Errorf("gemini key empty")
	}
	model := cfg.Model
	if model == "" {
		model = "gemini-2.0-flash"
	}
	endpoint := fmt.Sprintf(
		"https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s",
		url.PathEscape(model), url.QueryEscape(cfg.APIKey),
	)
	payload := map[string]interface{}{
		"system_instruction": map[string]interface{}{
			"parts": []map[string]string{{"text": system}},
		},
		"contents": []map[string]interface{}{
			{"role": "user", "parts": []map[string]string{{"text": user}}},
		},
		"generationConfig": map[string]interface{}{
			"temperature":     0.9,
			"maxOutputTokens": 900,
		},
	}
	raw, _ := json.Marshal(payload)
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("gemini HTTP %d: %s", resp.StatusCode, truncateRunes(string(data), 160))
	}
	var out struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return "", err
	}
	if len(out.Candidates) == 0 || len(out.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("пустой ответ gemini")
	}
	return strings.TrimSpace(out.Candidates[0].Content.Parts[0].Text), nil
}

func truncateRunes(s string, n int) string {
	s = strings.TrimSpace(s)
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n]) + "…"
}

// localLyricsDraft builds a usable Russian song draft without any external LLM.
func localLyricsDraft(idea, style string) string {
	idea = strings.TrimSpace(idea)
	style = strings.TrimSpace(style)
	hook := shortHook(idea)
	mood := "тихо и честно"
	if style != "" {
		mood = style
	}
	var b strings.Builder
	b.WriteString("[Куплет 1]\n")
	b.WriteString(fmt.Sprintf("Я всё ещё думаю про %s,\n", hook))
	b.WriteString("город молчит, а сердце не спит.\n")
	b.WriteString("Слова путаются в шуме витрин,\n")
	b.WriteString(fmt.Sprintf("но я храню эту мысль — %s.\n\n", hook))
	title := titleFirst(hook)
	b.WriteString("[Припев]\n")
	b.WriteString(fmt.Sprintf("%s — не отпускай,\n", title))
	b.WriteString("пусть ночь рисует нас снова.\n")
	b.WriteString(fmt.Sprintf("Мы звучим %s,\n", mood))
	b.WriteString("и эхо отвечает словом.\n\n")
	b.WriteString("[Куплет 2]\n")
	b.WriteString("Следы на стекле, чужие огни,\n")
	b.WriteString(fmt.Sprintf("в кармане билет и мечта про %s.\n", hook))
	b.WriteString("Я не прошу идеальный финал —\n")
	b.WriteString("мне хватит живого сигнала.\n\n")
	b.WriteString("[Припев]\n")
	b.WriteString(fmt.Sprintf("%s — не отпускай,\n", title))
	b.WriteString("пусть ночь рисует нас снова.\n")
	b.WriteString(fmt.Sprintf("Мы звучим %s,\n", mood))
	b.WriteString("и эхо отвечает словом.\n")
	return b.String()
}

func titleFirst(s string) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) == 0 {
		return s
	}
	return strings.ToUpper(string(r[0])) + string(r[1:])
}

func shortHook(idea string) string {
	idea = strings.TrimSpace(idea)
	if idea == "" {
		return "нас"
	}
	// take first meaningful chunk
	fields := strings.FieldsFunc(idea, func(r rune) bool {
		return r == ',' || r == '.' || r == ';' || r == '!' || r == '?' || r == '\n'
	})
	hook := idea
	if len(fields) > 0 {
		hook = strings.TrimSpace(fields[0])
	}
	hook = strings.TrimSpace(hook)
	if utf8.RuneCountInString(hook) > 42 {
		hook = string([]rune(hook)[:42])
	}
	if hook == "" {
		return "нас"
	}
	return hook
}
