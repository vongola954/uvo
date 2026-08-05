package services

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
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

	system := "Ты автор песен. Напиши УНИКАЛЬНЫЙ текст песни на русском под конкретную идею пользователя (куплет/припев), без пояснений и без клише «город молчит», 14–24 строк. Не повторяй одни и те же шаблоны."
	user := "Идея: " + idea
	if style != "" {
		user += "\nСтиль: " + style
	}
	seed := lyricsSeed(idea, style)

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
			"temperature": 1.1,
			"max_tokens":  900,
			"seed":        seed,
		}
		raw, _ := json.Marshal(body)
		text, err = callChatCompletions(cfg, raw)
		// Pollinations often 402 from cloud IPs — try plain GET text API, then Gemini, then local.
		if err != nil {
			if t, e2 := callPollinationsTextGET(system+"\n\n"+user, seed); e2 == nil && strings.TrimSpace(t) != "" {
				text, err = t, nil
			}
		}
		if err != nil && (strings.Contains(err.Error(), "402") || strings.Contains(err.Error(), "401") || strings.Contains(err.Error(), "429") || strings.Contains(err.Error(), "HTTP")) {
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

// callPollinationsTextGET hits the legacy free GET endpoint (sometimes works when chat/completions returns 402).
func callPollinationsTextGET(prompt string, seed uint64) (string, error) {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return "", fmt.Errorf("empty prompt")
	}
	if utf8.RuneCountInString(prompt) > 1200 {
		prompt = string([]rune(prompt)[:1200])
	}
	u := "https://text.pollinations.ai/" + url.PathEscape(prompt) +
		"?model=" + url.QueryEscape(defaultFreeLLMModel) +
		"&seed=" + fmt.Sprintf("%d", seed) +
		"&temperature=1"
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "text/plain, application/json;q=0.9,*/*;q=0.8")
	req.Header.Set("Referer", "https://pollinations.ai/")
	req.Header.Set("Origin", "https://pollinations.ai")
	client := &http.Client{Timeout: 45 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("pollinations GET HTTP %d", resp.StatusCode)
	}
	text := strings.TrimSpace(string(data))
	// Sometimes returns JSON wrapper
	if strings.HasPrefix(text, "{") {
		var wrap struct {
			Text    string `json:"text"`
			Content string `json:"content"`
			Choices []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			} `json:"choices"`
		}
		if json.Unmarshal(data, &wrap) == nil {
			if wrap.Text != "" {
				text = wrap.Text
			} else if wrap.Content != "" {
				text = wrap.Content
			} else if len(wrap.Choices) > 0 {
				text = wrap.Choices[0].Message.Content
			}
		}
	}
	text = strings.TrimSpace(text)
	if text == "" || strings.Contains(strings.ToLower(text), "payment required") {
		return "", fmt.Errorf("пустой ответ pollinations GET")
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

var lyricsDraftNonce atomic.Uint64

func lyricsSeed(idea, style string) uint64 {
	// nonce: Windows clock often coarse; counter guarantees unique drafts per call.
	n := lyricsDraftNonce.Add(1)
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s\x00%s\x00%d\x00%d", idea, style, time.Now().UnixNano(), n)))
	return binary.BigEndian.Uint64(sum[:8])
}

func pickLine(seed uint64, n int, lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	return lines[int((seed+uint64(n)*9973)%uint64(len(lines)))]
}

// localLyricsDraft builds a usable Russian song draft without any external LLM.
// Each call uses a fresh seed so the same idea does not yield an identical sheet.
func localLyricsDraft(idea, style string) string {
	idea = strings.TrimSpace(idea)
	style = strings.TrimSpace(style)
	seed := lyricsSeed(idea, style)
	hook := shortHook(idea)
	title := titleFirst(hook)
	keys := ideaKeywords(idea, 4)
	k1, k2, k3 := "эта ночь", "тишина", "дорога"
	if len(keys) > 0 {
		k1 = keys[0]
	}
	if len(keys) > 1 {
		k2 = keys[1]
	}
	if len(keys) > 2 {
		k3 = keys[2]
	}
	mood := detectMood(idea, style)
	moodPhrase := pickLine(seed, 1, moodPhrases[mood])
	if style != "" && utf8.RuneCountInString(style) <= 40 {
		moodPhrase = style
	}

	v1 := []string{
		fmt.Sprintf("Начинается с %s — и воздух другой,", k1),
		fmt.Sprintf("Я ловлю в голове отголосок: %s,", hook),
		fmt.Sprintf("Под шаги ложится тема про %s,", k1),
		fmt.Sprintf("Слова сами просятся: %s,", shortHook(k1+" "+k2)),
	}
	v1b := []string{
		fmt.Sprintf("между нами %s и лишний огонь.", k2),
		fmt.Sprintf("а за окном уже не спрятать %s.", k2),
		fmt.Sprintf("и пульс отвечает эхом на %s.", k2),
		fmt.Sprintf("пока %s держит меня на краю.", k2),
	}
	v1c := []string{
		fmt.Sprintf("Я собираю детали — %s, взгляд,", k3),
		fmt.Sprintf("Не спорю с судьбой, спорю с %s,", k3),
		fmt.Sprintf("Пусть мир шумит — у меня есть %s,", k3),
		fmt.Sprintf("Я пишу на ладони коротко: %s,", k3),
	}
	v1d := []string{
		fmt.Sprintf("и не отпускаю мысль про %s.", hook),
		fmt.Sprintf("пока не выскажу всё про %s.", hook),
		fmt.Sprintf("чтобы не растворить %s в пустых словах.", hook),
		fmt.Sprintf("и выбираю дорогу к %s.", hook),
	}

	chA := []string{
		fmt.Sprintf("%s — слышишь меня?", title),
		fmt.Sprintf("%s, не прячь огни,", title),
		fmt.Sprintf("Эй, %s — давай ещё раз,", strings.ToLower(title)),
		fmt.Sprintf("%s бьётся в такт,", title),
	}
	chB := []string{
		"мы выходим из тени на голос.",
		"сердце режет тишину пополам.",
		"я держу этот ритм до рассвета.",
		"пусть ветер допишет припев.",
	}
	chC := []string{
		fmt.Sprintf("Звучим %s —", moodPhrase),
		fmt.Sprintf("Поём %s —", moodPhrase),
		fmt.Sprintf("Дышим %s —", moodPhrase),
		fmt.Sprintf("Горим %s —", moodPhrase),
	}
	chD := []string{
		fmt.Sprintf("и %s становится хором.", k1),
		fmt.Sprintf("пока %s не станет мостом.", k2),
		fmt.Sprintf("и %s отвечает нам эхом.", hook),
		fmt.Sprintf("где %s — наш общий код.", k3),
	}

	v2 := []string{
		fmt.Sprintf("Вторая глава: %s на ладони,", k2),
		fmt.Sprintf("Я меняю маршрут — ближе к %s,", k1),
		fmt.Sprintf("Если спросишь зачем — отвечу: %s,", hook),
		fmt.Sprintf("Мы оставляем следы через %s,", k3),
	}
	v2b := []string{
		fmt.Sprintf("чужие советы тише, чем %s.", k3),
		fmt.Sprintf("и страх уже мельче, чем %s.", k2),
		fmt.Sprintf("а правда громче любого «потом» про %s.", hook),
		"я выбираю тепло вместо «никогда».",
	}
	v2c := []string{
		"Не нужен идеальный финал с титрами —",
		"Пусть будет криво, зато по-настоящему —",
		"Я не коллекционирую идеальные кадры —",
		"Хватит репетировать чужую жизнь —",
	}
	v2d := []string{
		fmt.Sprintf("мне хватит живого сигнала от %s.", hook),
		fmt.Sprintf("я беру %s как есть и иду дальше.", k1),
		fmt.Sprintf("оставляю в припеве только %s.", k2),
		fmt.Sprintf("и пою, пока звучит %s.", k3),
	}

	bridge := []string{
		fmt.Sprintf("Шёпотом: %s…\nгромче: мы здесь.", hook),
		fmt.Sprintf("Один вдох — и снова %s.", k1),
		fmt.Sprintf("Если выключишь свет — останется %s.", k2),
		fmt.Sprintf("Счёт: раз, два — и врывается %s.", title),
	}

	var b strings.Builder
	pack := int(seed % 3)
	b.WriteString("[Куплет 1]\n")
	b.WriteString(pickLine(seed, 10, v1) + "\n")
	b.WriteString(pickLine(seed, 11, v1b) + "\n")
	b.WriteString(pickLine(seed, 12, v1c) + "\n")
	b.WriteString(pickLine(seed, 13, v1d) + "\n\n")
	b.WriteString("[Припев]\n")
	b.WriteString(pickLine(seed, 20, chA) + "\n")
	b.WriteString(pickLine(seed, 21, chB) + "\n")
	b.WriteString(pickLine(seed, 22, chC) + "\n")
	b.WriteString(pickLine(seed, 23, chD) + "\n\n")
	if pack == 1 {
		b.WriteString("[Бридж]\n")
		b.WriteString(pickLine(seed, 30, bridge) + "\n\n")
	}
	b.WriteString("[Куплет 2]\n")
	b.WriteString(pickLine(seed, 40, v2) + "\n")
	b.WriteString(pickLine(seed, 41, v2b) + "\n")
	b.WriteString(pickLine(seed, 42, v2c) + "\n")
	b.WriteString(pickLine(seed, 43, v2d) + "\n\n")
	b.WriteString("[Припев]\n")
	b.WriteString(pickLine(seed, 50, chA) + "\n")
	b.WriteString(pickLine(seed, 51, chB) + "\n")
	b.WriteString(pickLine(seed, 52, chC) + "\n")
	b.WriteString(pickLine(seed, 53, chD) + "\n")
	if pack == 2 {
		b.WriteString("\n[Аутро]\n")
		b.WriteString(fmt.Sprintf("%s… ещё раз.\n", title))
		b.WriteString(pickLine(seed, 60, chB) + "\n")
	}
	return b.String()
}

var moodPhrases = map[string][]string{
	"love":    {"тепло и близко", "мягко, но честно", "как признание", "на грани шёпота"},
	"sad":     {"тише обычного", "сквозь дождь", "с надломом", "вполголоса"},
	"energy":  {"громко и прямо", "на полном ходу", "как удар бочки", "без тормозов"},
	"night":   {"ночным светом", "в неоне", "до рассвета", "в полутьме"},
	"road":    {"в движении", "километрами", "на скорости", "между станций"},
	"default": {"по-своему", "чисто и резко", "в своём темпе", "без масок"},
}

func detectMood(idea, style string) string {
	s := strings.ToLower(idea + " " + style)
	switch {
	case containsAny(s, "люб", "сердц", "поцел", "роман", "нежн", "ты и я"):
		return "love"
	case containsAny(s, "груст", "слёз", "боль", "один", "проща", "тоск", "дожд"):
		return "sad"
	case containsAny(s, "танц", "вечерин", "энерг", "драйв", "клуб", "громк", "хип"):
		return "energy"
	case containsAny(s, "ноч", "лун", "рассвет", "неон", "2:00", "темно"):
		return "night"
	case containsAny(s, "дорог", "поезд", "машин", "трасс", "путеше", "метро", "город"):
		return "road"
	default:
		return "default"
	}
}

func containsAny(s string, parts ...string) bool {
	for _, p := range parts {
		if strings.Contains(s, p) {
			return true
		}
	}
	return false
}

func ideaKeywords(idea string, n int) []string {
	stop := map[string]bool{
		"и": true, "в": true, "на": true, "с": true, "по": true, "к": true, "у": true,
		"о": true, "а": true, "но": true, "что": true, "как": true, "это": true, "я": true,
		"мы": true, "ты": true, "он": true, "она": true, "они": true, "для": true, "из": true,
		"про": true, "или": true, "же": true, "бы": true, "не": true, "да": true, "нет": true,
		"the": true, "a": true, "an": true, "to": true, "of": true, "and": true,
		"песня": true, "песню": true, "трек": true, "музыка": true,
	}
	fields := strings.FieldsFunc(strings.ToLower(idea), func(r rune) bool {
		return r == ',' || r == '.' || r == ';' || r == '!' || r == '?' || r == '\n' ||
			r == ' ' || r == '\t' || r == '"' || r == '\'' || r == '(' || r == ')' || r == '—' || r == '-'
	})
	out := make([]string, 0, n)
	seen := map[string]bool{}
	for _, w := range fields {
		w = strings.TrimSpace(w)
		if w == "" || stop[w] || utf8.RuneCountInString(w) < 3 {
			continue
		}
		if seen[w] {
			continue
		}
		seen[w] = true
		out = append(out, w)
		if len(out) >= n {
			break
		}
	}
	return out
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
