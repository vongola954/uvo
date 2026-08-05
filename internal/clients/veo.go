package clients

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// VeoClient — Google Veo via Gemini API (optional). Falls back to Kling in services.
type VeoClient struct {
	apiKey string
	model  string
	client *http.Client
}

func NewVeoClient() *VeoClient {
	key := strings.TrimSpace(os.Getenv("VEO_API_KEY"))
	if key == "" {
		key = strings.TrimSpace(os.Getenv("GEMINI_API_KEY"))
	}
	if key == "" {
		return nil
	}
	model := strings.TrimSpace(os.Getenv("VEO_MODEL"))
	if model == "" {
		model = "veo-2.0-generate-001"
	}
	return &VeoClient{apiKey: key, model: model, client: &http.Client{Timeout: 240 * time.Second}}
}

func (v *VeoClient) Enabled() bool { return v != nil && v.apiKey != "" }

// TextToVideo starts a Veo generation and polls until a downloadable URI appears.
func (v *VeoClient) TextToVideo(prompt string) (videoURL string, err error) {
	if !v.Enabled() {
		return "", fmt.Errorf("veo not configured")
	}
	if strings.TrimSpace(prompt) == "" {
		return "", fmt.Errorf("empty prompt")
	}
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:predictLongRunning?key=%s", v.model, v.apiKey)
	payload := map[string]interface{}{
		"instances": []map[string]string{{"prompt": prompt}},
		"parameters": map[string]interface{}{
			"aspectRatio":  "9:16",
			"sampleCount":  1,
		},
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := v.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("veo start %d: %s", resp.StatusCode, redactBody(b, 240))
	}
	var started struct {
		Name string `json:"name"`
	}
	_ = json.Unmarshal(b, &started)
	if started.Name == "" {
		return "", fmt.Errorf("veo: no operation name: %s", redactBody(b, 200))
	}
	return v.poll(started.Name)
}

func (v *VeoClient) poll(opName string) (string, error) {
	deadline := time.Now().Add(8 * time.Minute)
	for time.Now().Before(deadline) {
		u := "https://generativelanguage.googleapis.com/v1beta/" + opName + "?key=" + v.apiKey
		req, _ := http.NewRequest("GET", u, nil)
		resp, err := v.client.Do(req)
		if err != nil {
			time.Sleep(4 * time.Second)
			continue
		}
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		var op struct {
			Done     bool `json:"done"`
			Error    *struct {
				Message string `json:"message"`
			} `json:"error"`
			Response json.RawMessage `json:"response"`
		}
		_ = json.Unmarshal(b, &op)
		if op.Error != nil && op.Error.Message != "" {
			return "", fmt.Errorf("veo: %s", op.Error.Message)
		}
		if op.Done {
			if uri := findVideoURI(op.Response); uri != "" {
				return uri, nil
			}
			return "", fmt.Errorf("veo done but no video uri: %s", redactBody(b, 240))
		}
		time.Sleep(5 * time.Second)
	}
	return "", fmt.Errorf("veo timeout")
}

func findVideoURI(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var m map[string]interface{}
	if json.Unmarshal(raw, &m) != nil {
		return ""
	}
	// Walk common Veo response shapes.
	if gen, ok := m["generateVideoResponse"].(map[string]interface{}); ok {
		if arr, ok := gen["generatedSamples"].([]interface{}); ok && len(arr) > 0 {
			if sample, ok := arr[0].(map[string]interface{}); ok {
				if vid, ok := sample["video"].(map[string]interface{}); ok {
					if u, ok := vid["uri"].(string); ok {
						return u
					}
				}
			}
		}
	}
	s := string(raw)
	if i := strings.Index(s, "https://"); i >= 0 {
		rest := s[i:]
		end := strings.IndexAny(rest, "\"'\\s")
		if end > 0 {
			return rest[:end]
		}
	}
	return ""
}
