package clients

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/sirupsen/logrus"
)

// CreatePersona clones a singing voice from a publicly reachable audio_url (AceData /suno/voices).
func (c *AceDataClient) CreatePersona(audioURL, name, description string) (personaID string, err error) {
	if c.voicesURL == "" {
		return "", fmt.Errorf("acedata voices URL not configured")
	}
	if audioURL == "" {
		return "", fmt.Errorf("audio_url required")
	}
	payload := map[string]interface{}{
		"audio_url": audioURL,
	}
	if name != "" {
		payload["name"] = name
	}
	if description != "" {
		payload["description"] = description
	}
	body, err := c.postJSON(c.voicesURL, payload)
	if err != nil {
		return "", err
	}
	var resp struct {
		Success bool   `json:"success"`
		Error   string `json:"error"`
		Message string `json:"message"`
		Data    struct {
			PersonaID string `json:"persona_id"`
			Name      string `json:"name"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("decode persona response: %w", err)
	}
	if resp.Data.PersonaID == "" {
		if resp.Error != "" {
			return "", fmt.Errorf("persona create failed: %s", resp.Error)
		}
		return "", fmt.Errorf("persona_id missing: %s", redactBody(body, 200))
	}
	return resp.Data.PersonaID, nil
}

type UploadAudioResult struct {
	AudioID  string
	Title    string
	Lyric    string
	Style    string
	Duration float64
	AudioURL string
}

// UploadReference uploads a publicly reachable MP3 via AceData /suno/upload → audio_id.
func (c *AceDataClient) UploadReference(audioURL string) (*UploadAudioResult, error) {
	if c.uploadURL == "" {
		return nil, fmt.Errorf("acedata upload URL not configured")
	}
	if audioURL == "" {
		return nil, fmt.Errorf("audio_url required")
	}
	body, err := c.postJSON(c.uploadURL, map[string]interface{}{"audio_url": audioURL})
	if err != nil {
		return nil, err
	}
	var resp struct {
		Success bool   `json:"success"`
		Error   string `json:"error"`
		Message string `json:"message"`
		Data    struct {
			AudioID  string  `json:"audio_id"`
			Title    string  `json:"title"`
			Lyric    string  `json:"lyric"`
			Style    string  `json:"style"`
			Duration float64 `json:"duration"`
			AudioURL string  `json:"audio_url"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode upload response: %w", err)
	}
	if resp.Data.AudioID == "" {
		if resp.Error != "" {
			return nil, fmt.Errorf("upload failed: %s", resp.Error)
		}
		return nil, fmt.Errorf("audio_id missing: %s", redactBody(body, 200))
	}
	return &UploadAudioResult{
		AudioID:  resp.Data.AudioID,
		Title:    resp.Data.Title,
		Lyric:    resp.Data.Lyric,
		Style:    resp.Data.Style,
		Duration: resp.Data.Duration,
		AudioURL: resp.Data.AudioURL,
	}, nil
}

type CoverRequest struct {
	AudioID   string
	UploadURL string // alternative to AudioID for some AceData paths
	PersonaID string
	Prompt    string
	Style     string
	Lyric     string
	Title     string
	Model     string
}

// UploadCover creates a cover of an uploaded (or referenced) track, optionally with persona_id.
func (c *AceDataClient) UploadCover(req *CoverRequest) (*GenerateResponse, error) {
	model := c.model
	if req.Model != "" {
		model = req.Model
	}
	payload := map[string]interface{}{
		"action": "upload_cover",
		"model":  model,
		"async":  true,
	}
	if req.AudioID != "" {
		payload["audio_id"] = req.AudioID
	}
	if req.UploadURL != "" {
		payload["upload_url"] = req.UploadURL
	}
	if req.PersonaID != "" {
		payload["persona_id"] = req.PersonaID
	}
	if req.Prompt != "" {
		payload["prompt"] = req.Prompt
	}
	if req.Style != "" {
		payload["style"] = req.Style
	}
	if req.Lyric != "" {
		payload["custom"] = true
		payload["lyric"] = req.Lyric
	}
	if req.Title != "" {
		payload["title"] = req.Title
	}
	if req.AudioID == "" && req.UploadURL == "" {
		return nil, fmt.Errorf("audio_id or upload_url required")
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	logrus.WithField("payload", redactBody(jsonData, 256)).Debug("AceData upload_cover request")

	httpReq, err := http.NewRequest("POST", c.baseURL, bytes.NewReader(jsonData))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("upload_cover: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, ParseAceDataHTTPError(resp.StatusCode, body)
	}
	var createResp aceCreateResponse
	if err := json.Unmarshal(body, &createResp); err != nil {
		return nil, fmt.Errorf("decode upload_cover: %w", err)
	}
	if createResp.TaskID == "" {
		// Some responses return data inline
		var inline aceTaskResponse
		if json.Unmarshal(body, &inline) == nil && len(inline.Data) > 0 && inline.Data[0].AudioURL != "" {
			item := inline.Data[0]
			return &GenerateResponse{AudioURL: item.AudioURL, Title: item.Title, Duration: item.Duration}, nil
		}
		return nil, fmt.Errorf("upload_cover: no task_id: %s", redactBody(body, 200))
	}
	return c.pollTask(createResp.TaskID)
}

func (c *AceDataClient) postJSON(url string, payload map[string]interface{}) ([]byte, error) {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	logrus.WithField("payload", redactBody(jsonData, 256)).Debug("AceData POST " + url)
	httpReq, err := http.NewRequest("POST", url, bytes.NewReader(jsonData))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	logrus.WithFields(logrus.Fields{
		"status": resp.StatusCode,
		"body":   redactBody(body, 256),
	}).Debug("AceData POST response")
	if resp.StatusCode != http.StatusOK {
		return nil, ParseAceDataHTTPError(resp.StatusCode, body)
	}
	return body, nil
}
