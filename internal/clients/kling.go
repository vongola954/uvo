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
)

// KlingClient — AceData Kling image2video (portrait clip fallback without Hedra).
type KlingClient struct {
	apiKey string
	client *http.Client
}

func NewKlingClient(apiKey string) *KlingClient {
	if apiKey == "" {
		return nil
	}
	return &KlingClient{apiKey: apiKey, client: &http.Client{Timeout: 180 * time.Second}}
}

func (k *KlingClient) Enabled() bool { return k != nil && k.apiKey != "" }

// ImageToVideo animates a portrait (not true lip-sync; use Hedra for that).
func (k *KlingClient) ImageToVideo(imageURL, prompt string, durationSec int) (videoURL string, err error) {
	if !k.Enabled() {
		return "", fmt.Errorf("kling not configured")
	}
	if durationSec <= 0 {
		durationSec = 10
	}
	if prompt == "" {
		prompt = "Close-up portrait of the person singing passionately, natural facial expression, music video style, cinematic lighting"
	}
	payload := map[string]interface{}{
		"action":          "image2video",
		"model":           "kling-v2-6",
		"mode":            "pro",
		"start_image_url": imageURL,
		"prompt":          prompt,
		"duration":        durationSec,
		"aspect_ratio":    "9:16",
		"async":           true,
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", "https://api.acedata.cloud/kling/videos", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+k.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := k.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", ParseAceDataHTTPError(resp.StatusCode, b)
	}
	var create struct {
		Success  bool   `json:"success"`
		TaskID   string `json:"task_id"`
		VideoURL string `json:"video_url"`
		State    string `json:"state"`
		Error    string `json:"error"`
	}
	_ = json.Unmarshal(b, &create)
	if create.VideoURL != "" && (create.State == "succeed" || create.State == "succeeded" || create.State == "success") {
		return create.VideoURL, nil
	}
	if create.TaskID == "" {
		return "", fmt.Errorf("kling: no task_id: %s", redactBody(b, 200))
	}
	logrus.WithField("task_id", create.TaskID).Info("Kling portrait video started")
	return k.poll(create.TaskID)
}

func (k *KlingClient) poll(taskID string) (string, error) {
	deadline := time.Now().Add(8 * time.Minute)
	for time.Now().Before(deadline) {
		payload, _ := json.Marshal(map[string]interface{}{"id": taskID, "action": "retrieve"})
		req, err := http.NewRequest("POST", "https://api.acedata.cloud/kling/tasks", bytes.NewReader(payload))
		if err != nil {
			return "", err
		}
		req.Header.Set("Authorization", "Bearer "+k.apiKey)
		req.Header.Set("Content-Type", "application/json")
		resp, err := k.client.Do(req)
		if err != nil {
			time.Sleep(4 * time.Second)
			continue
		}
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		var tr struct {
			Success  bool   `json:"success"`
			VideoURL string `json:"video_url"`
			State    string `json:"state"`
			Error    string `json:"error"`
			Data     []struct {
				VideoURL string `json:"video_url"`
				State    string `json:"state"`
			} `json:"data"`
		}
		_ = json.Unmarshal(b, &tr)
		st := strings.ToLower(tr.State)
		url := tr.VideoURL
		if url == "" && len(tr.Data) > 0 {
			url = tr.Data[0].VideoURL
			if st == "" {
				st = strings.ToLower(tr.Data[0].State)
			}
		}
		if url != "" && (st == "" || st == "succeed" || st == "succeeded" || st == "success" || st == "completed") {
			return url, nil
		}
		if st == "failed" || st == "error" || tr.Error != "" {
			return "", fmt.Errorf("kling failed: %s", tr.Error)
		}
		time.Sleep(4 * time.Second)
	}
	return "", fmt.Errorf("kling timeout")
}
