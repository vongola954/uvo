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

// UpscaleClient enhances image resolution (Replicate Real-ESRGAN or AceData fal).
type UpscaleClient struct {
	replicateToken string
	aceKey         string
	client         *http.Client
}

func NewUpscaleClient(aceKey string) *UpscaleClient {
	tok := strings.TrimSpace(os.Getenv("REPLICATE_API_TOKEN"))
	if tok == "" && aceKey == "" {
		return nil
	}
	return &UpscaleClient{
		replicateToken: tok,
		aceKey:         aceKey,
		client:         &http.Client{Timeout: 180 * time.Second},
	}
}

func (u *UpscaleClient) Enabled() bool {
	return u != nil && (u.replicateToken != "" || u.aceKey != "")
}

// UpscaleURL takes a public HTTPS image URL and returns an upscaled image URL.
func (u *UpscaleClient) UpscaleURL(imageURL string) (string, error) {
	if !u.Enabled() {
		return "", fmt.Errorf("upscale not configured (REPLICATE_API_TOKEN or SUNO_API_KEY)")
	}
	if u.replicateToken != "" {
		return u.replicate(imageURL)
	}
	return u.aceData(imageURL)
}

func (u *UpscaleClient) replicate(imageURL string) (string, error) {
	payload := map[string]interface{}{
		"version": "f121d640bd286e1fdc67f9799164c1d5be36ff74576ee11c803ae5bdbf11ba7e",
		"input": map[string]interface{}{
			"image":  imageURL,
			"scale":  4,
			"face_enhance": true,
		},
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", "https://api.replicate.com/v1/predictions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Token "+u.replicateToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := u.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("replicate %d: %s", resp.StatusCode, redactBody(b, 200))
	}
	var created struct {
		ID     string `json:"id"`
		Status string `json:"status"`
		Output interface{} `json:"output"`
		URLs   struct {
			Get string `json:"get"`
		} `json:"urls"`
	}
	_ = json.Unmarshal(b, &created)
	getURL := created.URLs.Get
	if getURL == "" && created.ID != "" {
		getURL = "https://api.replicate.com/v1/predictions/" + created.ID
	}
	deadline := time.Now().Add(3 * time.Minute)
	for time.Now().Before(deadline) {
		if out := extractURL(created.Output); out != "" && (created.Status == "succeeded" || created.Status == "success") {
			return out, nil
		}
		req2, _ := http.NewRequest("GET", getURL, nil)
		req2.Header.Set("Authorization", "Token "+u.replicateToken)
		resp2, err := u.client.Do(req2)
		if err != nil {
			time.Sleep(2 * time.Second)
			continue
		}
		b2, _ := io.ReadAll(resp2.Body)
		resp2.Body.Close()
		_ = json.Unmarshal(b2, &created)
		st := strings.ToLower(created.Status)
		if out := extractURL(created.Output); out != "" && (st == "succeeded" || st == "success") {
			return out, nil
		}
		if st == "failed" || st == "canceled" {
			return "", fmt.Errorf("replicate failed: %s", redactBody(b2, 200))
		}
		time.Sleep(2 * time.Second)
	}
	return "", fmt.Errorf("replicate upscale timeout")
}

func (u *UpscaleClient) aceData(imageURL string) (string, error) {
	// AceData fal Real-ESRGAN style endpoint (best-effort; falls back with clear error).
	payload := map[string]interface{}{
		"image_url": imageURL,
		"scale":     4,
		"async":     false,
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", "https://api.acedata.cloud/fal/upscale", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+u.aceKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := u.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("upscale unavailable on AceData (%d). Задайте REPLICATE_API_TOKEN", resp.StatusCode)
	}
	var out struct {
		ImageURL string `json:"image_url"`
		URL      string `json:"url"`
		Output   string `json:"output"`
		Success  bool   `json:"success"`
	}
	_ = json.Unmarshal(b, &out)
	if out.ImageURL != "" {
		return out.ImageURL, nil
	}
	if out.URL != "" {
		return out.URL, nil
	}
	if out.Output != "" {
		return out.Output, nil
	}
	return "", fmt.Errorf("upscale empty response — задайте REPLICATE_API_TOKEN")
}

func extractURL(v interface{}) string {
	switch t := v.(type) {
	case string:
		return t
	case []interface{}:
		if len(t) > 0 {
			if s, ok := t[0].(string); ok {
				return s
			}
		}
	}
	return ""
}
