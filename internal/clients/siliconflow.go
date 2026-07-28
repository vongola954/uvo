package clients

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type SiliconFlowClient struct {
	apiKey string
	client *http.Client
}

func NewSiliconFlowClient(apiKey string) *SiliconFlowClient {
	return &SiliconFlowClient{
		apiKey: apiKey,
		client: &http.Client{Timeout: 60 * time.Second},
	}
}

type VoiceCloneRequest struct {
	Audio    string `json:"audio"` // base64
	Name     string `json:"name"`
	Language string `json:"language"`
}

type VoiceCloneResponse struct {
	VoiceID string `json:"voice_id"`
}

func (c *SiliconFlowClient) CloneVoice(audioData []byte, name string) (string, error) {
	if c.apiKey == "" {
		return "", fmt.Errorf("siliconflow api key not configured")
	}

	payload := VoiceCloneRequest{
		Audio:    base64.StdEncoding.EncodeToString(audioData),
		Name:     name,
		Language: "ru",
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", "https://api.siliconflow.cn/v1/voice/clone", bytes.NewReader(jsonData))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("siliconflow status %d: %s", resp.StatusCode, string(body))
	}

	var result VoiceCloneResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("decode: %w", err)
	}
	if result.VoiceID == "" {
		return "", fmt.Errorf("empty voice_id, body: %s", string(body))
	}
	return result.VoiceID, nil
}
