package clients

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/sirupsen/logrus"
)

type MAXClient struct {
	token   string
	baseURL string
	client  *http.Client
}

func NewMAXClient(token, baseURL string) *MAXClient {
	if baseURL == "" {
		baseURL = "https://platform-api2.max.ru"
	}
	return &MAXClient{
		token:   token,
		baseURL: baseURL,
		client:  &http.Client{Timeout: 100 * time.Second},
	}
}

func (m *MAXClient) Enabled() bool { return m.token != "" }

func (m *MAXClient) auth(req *http.Request) {
	// MAX platform: Authorization: <access_token> (no Bearer prefix)
	req.Header.Set("Authorization", m.token)
}

// SendMessage plain text to chat (or user dialog if chatID=0 and userID set).
func (m *MAXClient) SendMessage(chatID int64, text string) error {
	return m.send(0, chatID, map[string]interface{}{"text": text})
}

func (m *MAXClient) SendMessageToUser(userID, chatID int64, text string) error {
	return m.send(userID, chatID, map[string]interface{}{"text": text})
}

// SendStudio sends studio CTA + quick actions.
// open_app works only after mini-app URL is registered in MAX partner cabinet;
// otherwise we fall back to a link button «Запуск» (must not fail the whole reply).
func (m *MAXClient) SendStudio(chatID int64, text, studioURL string) error {
	return m.SendStudioTo(0, chatID, text, studioURL)
}

func (m *MAXClient) SendStudioTo(userID, chatID int64, text, studioURL string) error {
	if text == "" {
		text = "UVO — студия"
	}
	quick := []map[string]interface{}{
		{"type": "message", "text": "/generate"},
		{"type": "message", "text": "/credits"},
		{"type": "message", "text": "/help"},
	}

	// 1) Try in-MAX mini-app button (requires partner cabinet registration).
	if studioURL != "" {
		openApp := [][]map[string]interface{}{
			{{"type": "open_app", "text": "Запуск", "web_app": studioURL}},
			quick,
		}
		if err := m.sendKeyboard(userID, chatID, text, openApp); err == nil {
			return nil
		} else {
			logrus.WithError(err).Info("open_app unavailable — falling back to link Запуск")
		}
	}

	// 2) Reliable fallback: link + command buttons (always deliver a reply).
	buttons := [][]map[string]interface{}{quick}
	if studioURL != "" {
		buttons = [][]map[string]interface{}{
			{{"type": "link", "text": "Запуск", "url": studioURL}},
			quick,
		}
	}
	return m.sendKeyboard(userID, chatID, text, buttons)
}

func (m *MAXClient) sendKeyboard(userID, chatID int64, text string, buttons [][]map[string]interface{}) error {
	body := map[string]interface{}{
		"text": text,
		"attachments": []map[string]interface{}{
			{
				"type": "inline_keyboard",
				"payload": map[string]interface{}{
					"buttons": buttons,
				},
			},
		},
	}
	return m.send(userID, chatID, body)
}

// SetCommands registers slash-menu commands (fixes «Команды не найдены» in MAX UI).
func (m *MAXClient) SetCommands(commands []map[string]string) error {
	if m.token == "" {
		return fmt.Errorf("MAX token not configured")
	}
	raw, _ := json.Marshal(map[string]interface{}{"commands": commands})
	req, err := http.NewRequest("PATCH", m.baseURL+"/me/commands", bytes.NewReader(raw))
	if err != nil {
		return err
	}
	m.auth(req)
	req.Header.Set("Content-Type", "application/json")
	resp, err := m.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 300 {
		return fmt.Errorf("max set commands %d: %s", resp.StatusCode, string(b))
	}
	return nil
}

func (m *MAXClient) send(userID, chatID int64, payload map[string]interface{}) error {
	if m.token == "" {
		return fmt.Errorf("MAX token not configured")
	}
	if chatID == 0 && userID == 0 {
		return fmt.Errorf("max send: need chat_id or user_id")
	}
	q := url.Values{}
	if chatID != 0 {
		q.Set("chat_id", strconv.FormatInt(chatID, 10))
	}
	if userID != 0 {
		q.Set("user_id", strconv.FormatInt(userID, 10))
	}
	raw, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", m.baseURL+"/messages?"+q.Encode(), bytes.NewReader(raw))
	if err != nil {
		return err
	}
	m.auth(req)
	req.Header.Set("Content-Type", "application/json")
	resp, err := m.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 300 {
		logrus.WithFields(logrus.Fields{
			"status":  resp.StatusCode,
			"chat_id": chatID,
			"user_id": userID,
			"body":    truncateRunes(string(b), 300),
		}).Warn("MAX send message failed")
		return fmt.Errorf("max messages %d: %s", resp.StatusCode, string(b))
	}
	return nil
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

type MAXUpdatesResponse struct {
	Updates []MAXUpdate `json:"updates"`
	Marker  *int64      `json:"marker"`
}

type MAXUpdate struct {
	UpdateType string      `json:"update_type"`
	Timestamp  int64       `json:"timestamp"`
	ChatID     int64       `json:"chat_id"`
	User       *MAXUser    `json:"user"`
	Message    *MAXMessage `json:"message"`
}

type MAXUser struct {
	UserID   int64  `json:"user_id"`
	ID       int64  `json:"id"` // some payloads use id
	Name     string `json:"name"`
	Username string `json:"username"`
	IsBot    bool   `json:"is_bot"`
}

func (u *MAXUser) ID64() int64 {
	if u == nil {
		return 0
	}
	if u.UserID != 0 {
		return u.UserID
	}
	return u.ID
}

type MAXMessage struct {
	Sender *MAXUser `json:"sender"`
	Recipient *struct {
		ChatID int64 `json:"chat_id"`
		UserID int64 `json:"user_id"`
	} `json:"recipient"`
	Body *struct {
		Text string `json:"text"`
	} `json:"body"`
	Text   string `json:"text"`
	ChatID int64  `json:"chat_id"`
}

func (m *MAXClient) GetUpdates(marker *int64, timeoutSec int) (*MAXUpdatesResponse, error) {
	if m.token == "" {
		return nil, fmt.Errorf("MAX token not configured")
	}
	if timeoutSec <= 0 {
		timeoutSec = 30
	}
	q := url.Values{}
	q.Set("timeout", strconv.Itoa(timeoutSec))
	q.Set("limit", "100")
	q.Set("types", "message_created,bot_started")
	if marker != nil {
		q.Set("marker", strconv.FormatInt(*marker, 10))
	}
	req, err := http.NewRequest("GET", m.baseURL+"/updates?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	m.auth(req)
	resp, err := m.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("max updates %d: %s", resp.StatusCode, string(b))
	}
	var out MAXUpdatesResponse
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, fmt.Errorf("decode updates: %w body=%s", err, string(b))
	}
	return &out, nil
}

func (m *MAXClient) Me() (map[string]interface{}, error) {
	req, err := http.NewRequest("GET", m.baseURL+"/me", nil)
	if err != nil {
		return nil, err
	}
	m.auth(req)
	resp, err := m.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("max me %d: %s", resp.StatusCode, string(b))
	}
	var out map[string]interface{}
	_ = json.Unmarshal(b, &out)
	return out, nil
}
