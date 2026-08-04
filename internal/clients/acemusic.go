package clients

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"uvo/internal/config"
)

// AceMusicClient talks to ACE-Step cloud (acemusic.ai) via OpenRouter-compatible
// POST /v1/chat/completions (audio returned as data: URL base64).
type AceMusicClient struct {
	apiKey  string
	baseURL string
	model   string
	timeout time.Duration
	client  *http.Client
}

func NewAceMusicClient(cfg *config.Config) *AceMusicClient {
	if cfg == nil || strings.TrimSpace(cfg.AceMusicAPIKey) == "" {
		return nil
	}
	base := strings.TrimRight(cfg.AceMusicBaseURL, "/")
	if base == "" {
		base = "https://api.acemusic.ai"
	}
	model := strings.TrimSpace(cfg.AceMusicModel)
	if model == "" {
		model = "acemusic/acestep-v1.5-turbo"
	}
	timeoutSec := cfg.AceMusicTimeout
	if timeoutSec <= 0 {
		timeoutSec = 600
	}
	to := time.Duration(timeoutSec) * time.Second
	return &AceMusicClient{
		apiKey:  cfg.AceMusicAPIKey,
		baseURL: base,
		model:   model,
		timeout: to,
		client:  &http.Client{Timeout: to},
	}
}

func (c *AceMusicClient) Enabled() bool {
	return c != nil && c.apiKey != ""
}

func (c *AceMusicClient) Generate(req *GenerateRequest) (*GenerateResponse, error) {
	clips, err := c.GenerateAll(req)
	if err != nil {
		return nil, err
	}
	return clips[0], nil
}

func (c *AceMusicClient) GenerateAll(req *GenerateRequest) ([]*GenerateResponse, error) {
	if !c.Enabled() {
		return nil, fmt.Errorf("acemusic not configured")
	}
	if req == nil {
		return nil, fmt.Errorf("nil request")
	}

	content := buildAceMusicContent(req)
	sampleMode := !req.Instrumental && strings.TrimSpace(req.Lyric) == ""
	payload := map[string]interface{}{
		"model":            c.model,
		"messages":         []map[string]string{{"role": "user", "content": content}},
		"stream":           false,
		"thinking":         false,
		"use_format":       false,
		"sample_mode":      sampleMode,
		"use_cot_caption":  false,
		"use_cot_language": false,
		"audio_config": map[string]interface{}{
			"format":         "mp3",
			"vocal_language": "ru",
			"duration":       clampAceMusicDuration(req.Duration),
		},
	}
	if req.Instrumental {
		// Caption-mode instrumental: empty/inst lyrics tag already in content.
		payload["sample_mode"] = false
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}
	logrus.WithField("payload", redactBody(body, 280)).Info("AceMusic create request")

	httpReq, err := http.NewRequest("POST", c.baseURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")

	resp, err := c.client.Do(httpReq)
	if err != nil {
		if isTimeoutErr(err) {
			return nil, &ProviderError{
				Code:    "provider_timeout",
				Message: "AceMusic не ответил вовремя. Выберите длину 1 мин и попробуйте снова.",
				Status:  504,
			}
		}
		return nil, fmt.Errorf("acemusic request: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 80<<20))
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		logrus.WithFields(logrus.Fields{
			"status": resp.StatusCode,
			"body":   redactBody(raw, 256),
		}).Warn("AceMusic create failed")
		return nil, ParseAceMusicHTTPError(resp.StatusCode, raw)
	}

	clips, metaTitle, metaLyric, err := parseAceMusicCompletion(raw)
	if err != nil {
		return nil, err
	}
	for _, clip := range clips {
		if clip.Title == "" {
			clip.Title = metaTitle
		}
		if clip.Title == "" {
			clip.Title = req.Title
		}
		if clip.Lyric == "" {
			clip.Lyric = metaLyric
		}
		if clip.Lyric == "" {
			clip.Lyric = req.Lyric
		}
		if err := c.materializeAudio(clip); err != nil {
			return nil, fmt.Errorf("materialize audio: %w", err)
		}
	}
	bytesLen := 0
	if len(clips) > 0 {
		bytesLen = len(clips[0].AudioBytes)
	}
	logrus.WithFields(logrus.Fields{"clips": len(clips), "audio_bytes": bytesLen}).Info("AceMusic create ok")
	return clips, nil
}

func buildAceMusicContent(req *GenerateRequest) string {
	prompt := strings.TrimSpace(req.Prompt)
	if req.Style != "" {
		if prompt == "" {
			prompt = req.Style
		} else {
			prompt = prompt + ", " + req.Style
		}
	}
	lyric := strings.TrimSpace(req.Lyric)
	if req.Instrumental {
		if lyric == "" {
			lyric = "[inst]"
		}
		return fmt.Sprintf("<prompt>%s</prompt><lyrics>%s</lyrics>", prompt, lyric)
	}
	if lyric != "" {
		return fmt.Sprintf("<prompt>%s</prompt><lyrics>%s</lyrics>", prompt, lyric)
	}
	// simple / idea mode
	if prompt == "" {
		prompt = "short instrumental track"
	}
	return prompt
}

func clampAceMusicDuration(sec int) int {
	// Free AceMusic cloud often stalls on long tracks; keep requests short.
	maxSec := 90
	if v := strings.TrimSpace(os.Getenv("ACEMUSIC_MAX_DURATION")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 15 && n <= 240 {
			maxSec = n
		}
	}
	if sec <= 0 {
		sec = 60
	}
	if sec > maxSec {
		sec = maxSec
	}
	if sec < 15 {
		sec = 15
	}
	return sec
}

func isTimeoutErr(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, os.ErrDeadlineExceeded) {
		return true
	}
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "timeout") || strings.Contains(msg, "deadline exceeded")
}

func parseAceMusicCompletion(raw []byte) (clips []*GenerateResponse, title, lyric string, err error) {
	var envelope struct {
		ID      string `json:"id"`
		Choices []struct {
			Message struct {
				Content      string          `json:"content"`
				Audio        json.RawMessage `json:"audio"`
				FinishReason string          `json:"finish_reason"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Error *struct {
			Message string `json:"message"`
			Code    string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, "", "", fmt.Errorf("decode completion: %w", err)
	}
	if envelope.Error != nil && envelope.Error.Message != "" {
		return nil, "", "", &ProviderError{
			Code:    "provider_error",
			Message: "AceMusic: " + truncate(envelope.Error.Message, 200),
			Status:  502,
		}
	}
	if len(envelope.Choices) == 0 {
		return nil, "", "", fmt.Errorf("acemusic: empty choices, body: %s", redactBody(raw, 200))
	}
	msg := envelope.Choices[0].Message
	if msg.FinishReason == "error" || envelope.Choices[0].FinishReason == "error" {
		return nil, "", "", fmt.Errorf("acemusic generation error: %s", truncate(msg.Content, 200))
	}
	title, lyric = parseAceMusicMeta(msg.Content)
	urls := extractAceMusicAudioRefs(msg.Audio)
	for i, u := range urls {
		clips = append(clips, &GenerateResponse{
			AudioURL: u,
			AudioID:  fmt.Sprintf("%s-%d", envelope.ID, i),
			TaskID:   envelope.ID,
			Title:    title,
			Lyric:    lyric,
		})
	}
	if len(clips) == 0 {
		return nil, title, lyric, fmt.Errorf("acemusic: no audio in response (content=%s)", truncate(msg.Content, 120))
	}
	return clips, title, lyric, nil
}

// extractAceMusicAudioRefs accepts several AceMusic/OpenRouter shapes.
func extractAceMusicAudioRefs(raw json.RawMessage) []string {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var out []string
	add := func(s string) {
		s = strings.TrimSpace(s)
		if s == "" {
			return
		}
		out = append(out, s)
	}
	// Array of objects / strings
	var arr []json.RawMessage
	if err := json.Unmarshal(raw, &arr); err == nil {
		for _, item := range arr {
			var s string
			if json.Unmarshal(item, &s) == nil {
				add(s)
				continue
			}
			var obj map[string]interface{}
			if json.Unmarshal(item, &obj) != nil {
				continue
			}
			add(stringFromAny(obj["url"]))
			add(stringFromAny(obj["data"]))
			add(stringFromAny(obj["file"]))
			if au, ok := obj["audio_url"]; ok {
				switch v := au.(type) {
				case string:
					add(v)
				case map[string]interface{}:
					add(stringFromAny(v["url"]))
				}
			}
		}
		return out
	}
	// Single object
	var obj map[string]interface{}
	if json.Unmarshal(raw, &obj) == nil {
		add(stringFromAny(obj["url"]))
		add(stringFromAny(obj["data"]))
		if au, ok := obj["audio_url"].(map[string]interface{}); ok {
			add(stringFromAny(au["url"]))
		}
	}
	return out
}

func stringFromAny(v interface{}) string {
	s, _ := v.(string)
	return strings.TrimSpace(s)
}

func audioURLKind(u string) string {
	switch {
	case strings.HasPrefix(u, "data:"):
		return "data"
	case strings.HasPrefix(u, "https://"), strings.HasPrefix(u, "http://"):
		return "http"
	case strings.HasPrefix(u, "/"):
		return "path"
	case looksLikeBase64Audio(u):
		return "raw_b64"
	default:
		return "other"
	}
}

func looksLikeBase64Audio(s string) bool {
	if len(s) < 64 || strings.Contains(s, "://") || strings.Contains(s, " ") {
		return false
	}
	for _, r := range s[:64] {
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') ||
			r == '+' || r == '/' || r == '=' || r == '-' || r == '_' {
			continue
		}
		return false
	}
	return true
}

// materializeAudio loads audio into AudioBytes (no giant data: URL round-trip).
func (c *AceMusicClient) materializeAudio(clip *GenerateResponse) error {
	if clip == nil {
		return fmt.Errorf("nil clip")
	}
	u := strings.TrimSpace(clip.AudioURL)
	if u == "" {
		return fmt.Errorf("empty audio url")
	}
	var b []byte
	var err error
	switch {
	case strings.HasPrefix(u, "data:"):
		b, _, err = decodeDataURL(u)
	case looksLikeBase64Audio(u):
		b, err = decodeBase64Loose(u)
	default:
		b, err = c.fetchAudioBytes(u)
	}
	if err != nil {
		return err
	}
	if len(b) < 64 {
		return fmt.Errorf("audio payload too small (%d bytes)", len(b))
	}
	// Reject obvious non-audio HTML/JSON error bodies.
	head := strings.ToLower(string(b[:min(64, len(b))]))
	if strings.Contains(head, "<!doctype") || strings.Contains(head, "<html") ||
		strings.HasPrefix(strings.TrimSpace(head), "{") {
		return fmt.Errorf("audio payload looks like text/html or json, not audio")
	}
	clip.AudioBytes = b
	clip.AudioURL = "" // prefer in-memory bytes in GenerationService
	return nil
}

func (c *AceMusicClient) fetchAudioBytes(u string) ([]byte, error) {
	full := u
	if strings.HasPrefix(u, "/") {
		full = c.baseURL + u
	}
	parsed, err := url.Parse(full)
	if err != nil {
		return nil, fmt.Errorf("bad audio url: %w", err)
	}
	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return nil, fmt.Errorf("unsupported audio url scheme")
	}
	req, err := http.NewRequest("GET", full, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "*/*")
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch audio: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 40<<20))
	if err != nil {
		return nil, fmt.Errorf("read audio: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch audio HTTP %d: %s", resp.StatusCode, truncate(string(body), 160))
	}
	return body, nil
}

func decodeDataURL(raw string) (data []byte, mime string, err error) {
	comma := strings.Index(raw, ",")
	if comma < 0 {
		return nil, "", fmt.Errorf("bad data url")
	}
	meta := raw[:comma]
	payload := raw[comma+1:]
	mime = "audio/mpeg"
	if strings.HasPrefix(meta, "data:") {
		rest := strings.TrimPrefix(meta, "data:")
		rest = strings.Split(rest, ";")[0]
		if rest != "" {
			mime = rest
		}
	}
	if !strings.Contains(meta, ";base64") {
		return nil, "", fmt.Errorf("data url must be base64")
	}
	b, err := decodeBase64Loose(payload)
	if err != nil {
		return nil, "", err
	}
	return b, mime, nil
}

func decodeBase64Loose(s string) ([]byte, error) {
	s = strings.Map(func(r rune) rune {
		switch r {
		case '\n', '\r', '\t', ' ':
			return -1
		default:
			return r
		}
	}, s)
	encodings := []*base64.Encoding{
		base64.StdEncoding,
		base64.RawStdEncoding,
		base64.URLEncoding,
		base64.RawURLEncoding,
	}
	var last error
	for _, enc := range encodings {
		b, err := enc.DecodeString(s)
		if err == nil {
			return b, nil
		}
		last = err
	}
	// pad to multiple of 4
	if m := len(s) % 4; m != 0 {
		s2 := s + strings.Repeat("=", 4-m)
		if b, err := base64.StdEncoding.DecodeString(s2); err == nil {
			return b, nil
		}
	}
	if last == nil {
		last = fmt.Errorf("invalid base64")
	}
	return nil, last
}

func parseAceMusicMeta(content string) (title, lyric string) {
	// Best-effort parse of markdown-ish assistant content.
	lines := strings.Split(content, "\n")
	var lyricLines []string
	inLyrics := false
	for _, ln := range lines {
		trim := strings.TrimSpace(ln)
		low := strings.ToLower(trim)
		if strings.Contains(low, "**caption:**") || strings.HasPrefix(low, "caption:") {
			parts := strings.SplitN(trim, ":", 2)
			if len(parts) == 2 && title == "" {
				title = strings.TrimSpace(strings.Trim(parts[1], "* "))
			}
		}
		if strings.Contains(low, "## lyrics") {
			inLyrics = true
			continue
		}
		if inLyrics {
			if strings.HasPrefix(trim, "## ") {
				break
			}
			lyricLines = append(lyricLines, ln)
		}
	}
	lyric = strings.TrimSpace(strings.Join(lyricLines, "\n"))
	if title == "" {
		title = "AceMusic track"
	}
	return title, lyric
}

// ParseAceMusicHTTPError maps AceMusic/Cloudflare HTTP failures.
func ParseAceMusicHTTPError(status int, body []byte) error {
	msg := string(body)
	var payload struct {
		Error struct {
			Message string `json:"message"`
			Code    string `json:"code"`
		} `json:"error"`
		Message string `json:"message"`
	}
	_ = json.Unmarshal(body, &payload)
	if payload.Error.Message != "" {
		msg = payload.Error.Message
	} else if payload.Message != "" {
		msg = payload.Message
	}
	lower := strings.ToLower(msg + " " + payload.Error.Code)
	switch {
	case status == 401 || status == 403:
		if strings.Contains(lower, "balance") || strings.Contains(lower, "quota") ||
			strings.Contains(lower, "used_up") || strings.Contains(lower, "insufficient") {
			return &ProviderError{
				Code:    "provider_balance_empty",
				Message: "Баланс AceMusic исчерпан. Пополните на https://acemusic.ai",
				Status:  503,
			}
		}
		return &ProviderError{
			Code:    "provider_auth",
			Message: "Ошибка авторизации AceMusic (проверьте ACEMUSIC_API_KEY)",
			Status:  502,
		}
	case status == 429:
		return &ProviderError{Code: "provider_rate_limit", Message: "AceMusic rate limit", Status: 429}
	default:
		return &ProviderError{
			Code:    "provider_error",
			Message: fmt.Sprintf("AceMusic HTTP %d: %s", status, truncate(msg, 200)),
			Status:  502,
		}
	}
}
