package clients

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

type MAXClient struct {
	token    string
	baseURL  string
	username string // bot username for open_app deep links (max.ru/<user>?startapp)
	client   *http.Client
}

func NewMAXClient(token, baseURL string) *MAXClient {
	if baseURL == "" {
		baseURL = "https://platform-api2.max.ru"
	}
	return &MAXClient{
		token:   token,
		baseURL: baseURL,
		client:  HTTPClient(100 * time.Second),
	}
}

func (m *MAXClient) Enabled() bool { return m.token != "" }

// SetUsername sets bot @username used for open_app (without @).
func (m *MAXClient) SetUsername(username string) {
	m.username = strings.TrimPrefix(strings.TrimSpace(username), "@")
}

// StartAppURL is the in-MAX mini-app deep link required by open_app.
func (m *MAXClient) StartAppURL() string {
	if m.username == "" {
		return ""
	}
	return "https://max.ru/" + m.username + "?startapp"
}

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
// open_app requires partner-cabinet mini-app + web_app=https://max.ru/<bot>?startapp
// (direct https studio URL returns 404 from MAX API).
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

	// 1) In-MAX window via open_app deep link.
	if start := m.StartAppURL(); start != "" {
		openApp := [][]map[string]interface{}{
			{{"type": "open_app", "text": "Запуск", "web_app": start}},
			quick,
		}
		if err := m.sendKeyboard(userID, chatID, text, openApp); err != nil {
			logrus.WithError(err).Info("open_app failed — falling back to link Запуск")
		} else {
			return nil
		}
	}

	// 2) Reliable fallback: https link + command buttons.
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
	UserID     int64       `json:"user_id"` // top-level on bot_started (2026 payloads)
	User       *MAXUser    `json:"user"`
	Message    *MAXMessage `json:"message"`
	Payload    string      `json:"payload"`
}

type MAXUser struct {
	UserID    int64  `json:"user_id"`
	ID        int64  `json:"id"` // some payloads use id
	Name      string `json:"name"`
	FirstName string `json:"first_name"`
	Username  string `json:"username"`
	IsBot     bool   `json:"is_bot"`
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
	if u, ok := out["username"].(string); ok && u != "" {
		m.SetUsername(u)
	}
	return out, nil
}

// Username returns bot @username used for open_app (without @).
func (m *MAXClient) Username() string { return m.username }

type MAXSubscription struct {
	URL         string   `json:"url"`
	UpdateTypes []string `json:"update_types"`
}

// ListSubscriptions returns active webhook subscriptions.
func (m *MAXClient) ListSubscriptions() ([]MAXSubscription, error) {
	if m.token == "" {
		return nil, fmt.Errorf("MAX token not configured")
	}
	req, err := http.NewRequest("GET", m.baseURL+"/subscriptions", nil)
	if err != nil {
		return nil, err
	}
	m.auth(req)
	resp, err := m.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("max subscriptions %d: %s", resp.StatusCode, string(b))
	}
	var out struct {
		Subscriptions []MAXSubscription `json:"subscriptions"`
	}
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	return out.Subscriptions, nil
}

// ClearSubscriptions removes webhook subscriptions so GET /updates (long-poll) receives events.
func (m *MAXClient) ClearSubscriptions() error {
	subs, err := m.ListSubscriptions()
	if err != nil {
		return err
	}
	for _, s := range subs {
		u := strings.TrimSpace(s.URL)
		if u == "" {
			continue
		}
		req, err := http.NewRequest("DELETE", m.baseURL+"/subscriptions?url="+url.QueryEscape(u), nil)
		if err != nil {
			return err
		}
		m.auth(req)
		resp, err := m.client.Do(req)
		if err != nil {
			return err
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		if resp.StatusCode >= 300 {
			logrus.WithField("status", resp.StatusCode).Warn("MAX delete subscription failed")
		}
	}
	return nil
}
