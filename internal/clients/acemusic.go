package clients

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
	}
	logrus.WithField("clips", len(clips)).Info("AceMusic create ok")
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
	if sec <= 0 {
		return 60
	}
	if sec > 240 {
		return 240
	}
	if sec < 15 {
		return 15
	}
	return sec
}

type aceMusicCompletion struct {
	ID      string `json:"id"`
	Choices []struct {
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
			Audio   []struct {
				Type     string `json:"type"`
				AudioURL struct {
					URL string `json:"url"`
				} `json:"audio_url"`
			} `json:"audio"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Code    string `json:"code"`
	} `json:"error"`
}

func parseAceMusicCompletion(raw []byte) (clips []*GenerateResponse, title, lyric string, err error) {
	var parsed aceMusicCompletion
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, "", "", fmt.Errorf("decode completion: %w", err)
	}
	if parsed.Error != nil && parsed.Error.Message != "" {
		return nil, "", "", &ProviderError{
			Code:    "provider_error",
			Message: "AceMusic: " + truncate(parsed.Error.Message, 200),
			Status:  502,
		}
	}
	if len(parsed.Choices) == 0 {
		return nil, "", "", fmt.Errorf("acemusic: empty choices, body: %s", redactBody(raw, 200))
	}
	msg := parsed.Choices[0].Message
	title, lyric = parseAceMusicMeta(msg.Content)
	for i, a := range msg.Audio {
		u := strings.TrimSpace(a.AudioURL.URL)
		if u == "" {
			continue
		}
		clips = append(clips, &GenerateResponse{
			AudioURL: u,
			AudioID:  fmt.Sprintf("%s-%d", parsed.ID, i),
			TaskID:   parsed.ID,
			Title:    title,
			Lyric:    lyric,
		})
	}
	if len(clips) == 0 {
		return nil, title, lyric, fmt.Errorf("acemusic: no audio in response")
	}
	return clips, title, lyric, nil
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
