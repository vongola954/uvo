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
	req.Header.Set("Authorization", m.token)
}

// SendMessage plain text
func (m *MAXClient) SendMessage(chatID int64, text string) error {
	return m.send(0, chatID, map[string]interface{}{"text": text})
}

// SendStudio opens web UI via inline link button (MAX keyboard)
func (m *MAXClient) SendStudio(chatID int64, text, studioURL string) error {
	if studioURL == "" {
		return m.SendMessage(chatID, text)
	}
	body := map[string]interface{}{
		"text": text,
		"attachments": []map[string]interface{}{
			{
				"type": "inline_keyboard",
				"payload": map[string]interface{}{
					"buttons": [][]map[string]interface{}{
						{
							{"type": "link", "text": "🎵 Открыть веб-студию", "url": studioURL},
						},
						{
							{"type": "message", "text": "⚡ /generate"},
							{"type": "message", "text": "❓ /help"},
						},
					},
				},
			},
		},
	}
	return m.send(0, chatID, body)
}

func (m *MAXClient) send(userID, chatID int64, payload map[string]interface{}) error {
	if m.token == "" {
		return fmt.Errorf("MAX token not configured")
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
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return fmt.Errorf("max messages %d: %s", resp.StatusCode, string(b))
	}
	return nil
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
	Name     string `json:"name"`
	Username string `json:"username"`
	IsBot    bool   `json:"is_bot"`
}

type MAXMessage struct {
	Sender *MAXUser `json:"sender"`
	Body   *struct {
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
	b, _ := io.ReadAll(resp.Body)
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
