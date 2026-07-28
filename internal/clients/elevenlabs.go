package clients

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"
)

// ElevenLabs API — voice clone + TTS
// Docs: https://elevenlabs.io/docs/api-reference
type ElevenLabsClient struct {
	apiKey string
	client *http.Client
}

func NewElevenLabsClient(apiKey string) *ElevenLabsClient {
	return &ElevenLabsClient{
		apiKey: apiKey,
		client: &http.Client{Timeout: 120 * time.Second},
	}
}

func (c *ElevenLabsClient) Enabled() bool {
	return c.apiKey != ""
}

// CloneVoice uploads audio samples and creates an instant voice clone.
// Returns voice_id.
func (c *ElevenLabsClient) CloneVoice(audioData []byte, name string) (string, error) {
	if !c.Enabled() {
		return "", fmt.Errorf("elevenlabs api key not configured")
	}

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	_ = w.WriteField("name", name)
	_ = w.WriteField("description", "UVO voice clone")
	part, err := w.CreateFormFile("files", "sample.mp3")
	if err != nil {
		return "", err
	}
	if _, err := part.Write(audioData); err != nil {
		return "", err
	}
	_ = w.Close()

	req, err := http.NewRequest("POST", "https://api.elevenlabs.io/v1/voices/add", &buf)
	if err != nil {
		return "", err
	}
	req.Header.Set("xi-api-key", c.apiKey)
	req.Header.Set("Content-Type", w.FormDataContentType())

	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("elevenlabs clone %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		VoiceID string `json:"voice_id"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("decode: %w body=%s", err, string(body))
	}
	if result.VoiceID == "" {
		return "", fmt.Errorf("empty voice_id: %s", string(body))
	}
	return result.VoiceID, nil
}

// TextToSpeech synthesizes speech with a voice_id.
func (c *ElevenLabsClient) TextToSpeech(voiceID, text, modelID string) ([]byte, error) {
	if !c.Enabled() {
		return nil, fmt.Errorf("elevenlabs api key not configured")
	}
	if modelID == "" {
		modelID = "eleven_multilingual_v2"
	}
	payload := map[string]interface{}{
		"text":     text,
		"model_id": modelID,
	}
	jsonData, _ := json.Marshal(payload)
	url := fmt.Sprintf("https://api.elevenlabs.io/v1/text-to-speech/%s", voiceID)
	req, err := http.NewRequest("POST", url, bytes.NewReader(jsonData))
	if err != nil {
		return nil, err
	}
	req.Header.Set("xi-api-key", c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "audio/mpeg")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("elevenlabs tts %d: %s", resp.StatusCode, string(body))
	}
	return body, nil
}

// ListVoices returns available voices.
func (c *ElevenLabsClient) ListVoices() ([]map[string]string, error) {
	if !c.Enabled() {
		return nil, fmt.Errorf("elevenlabs api key not configured")
	}
	req, err := http.NewRequest("GET", "https://api.elevenlabs.io/v1/voices", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("xi-api-key", c.apiKey)
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("elevenlabs voices %d: %s", resp.StatusCode, string(body))
	}
	var result struct {
		Voices []struct {
			VoiceID string `json:"voice_id"`
			Name    string `json:"name"`
		} `json:"voices"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	out := make([]map[string]string, 0, len(result.Voices))
	for _, v := range result.Voices {
		out = append(out, map[string]string{"voice_id": v.VoiceID, "name": v.Name})
	}
	return out, nil
}
