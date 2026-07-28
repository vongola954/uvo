package clients

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/sirupsen/logrus"
)

// HedraClient — singing portrait / avatar lip-sync (optional HEDRA_API_KEY).
type HedraClient struct {
	apiKey string
	base   string
	client *http.Client
}

func NewHedraClient(apiKey string) *HedraClient {
	if apiKey == "" {
		return nil
	}
	return &HedraClient{
		apiKey: apiKey,
		base:   "https://api.hedra.com/web-app/public",
		client: &http.Client{Timeout: 120 * time.Second},
	}
}

func (h *HedraClient) Enabled() bool { return h != nil && h.apiKey != "" }

type hederaAsset struct {
	ID string `json:"id"`
}

func (h *HedraClient) createAsset(name, typ string) (string, error) {
	body, _ := json.Marshal(map[string]string{"name": name, "type": typ})
	req, err := http.NewRequest("POST", h.base+"/assets", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", h.apiKey)
	resp, err := h.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("hedra create asset %d: %s", resp.StatusCode, redactBody(b, 200))
	}
	var a hederaAsset
	if err := json.Unmarshal(b, &a); err != nil || a.ID == "" {
		// some responses nest under data
		var wrap struct {
			ID   string `json:"id"`
			Data struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		_ = json.Unmarshal(b, &wrap)
		a.ID = wrap.ID
		if a.ID == "" {
			a.ID = wrap.Data.ID
		}
	}
	if a.ID == "" {
		return "", fmt.Errorf("hedra asset id missing: %s", redactBody(b, 200))
	}
	return a.ID, nil
}

func (h *HedraClient) uploadAsset(assetID, filePath string) error {
	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer f.Close()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		return err
	}
	if _, err := io.Copy(part, f); err != nil {
		return err
	}
	_ = w.Close()
	req, err := http.NewRequest("POST", h.base+"/assets/"+assetID+"/upload", &buf)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("X-API-Key", h.apiKey)
	resp, err := h.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return fmt.Errorf("hedra upload %d: %s", resp.StatusCode, redactBody(b, 200))
	}
	return nil
}

// GenerateAvatar creates lip-sync video from portrait image + audio file paths.
func (h *HedraClient) GenerateAvatar(imagePath, audioPath, prompt string) (videoURL string, err error) {
	if !h.Enabled() {
		return "", fmt.Errorf("hedra not configured")
	}
	imgID, err := h.createAsset(filepath.Base(imagePath), "image")
	if err != nil {
		return "", err
	}
	if err := h.uploadAsset(imgID, imagePath); err != nil {
		return "", err
	}
	audID, err := h.createAsset(filepath.Base(audioPath), "audio")
	if err != nil {
		return "", err
	}
	if err := h.uploadAsset(audID, audioPath); err != nil {
		return "", err
	}
	if prompt == "" {
		prompt = "A person singing passionately to the camera, expressive performance"
	}
	payload := map[string]interface{}{
		"type":             "video",
		"model_slug":       "together/hedra-avatar",
		"start_keyframe_id": imgID,
		"audio_id":         audID,
		"generated_video_inputs": map[string]interface{}{
			"text_prompt":  prompt,
			"aspect_ratio": "9:16",
			"resolution":   "720p",
		},
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", h.base+"/generations", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", h.apiKey)
	resp, err := h.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("hedra generate %d: %s", resp.StatusCode, redactBody(b, 200))
	}
	var gen struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(b, &gen)
	if gen.ID == "" {
		var wrap struct {
			Data struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		_ = json.Unmarshal(b, &wrap)
		gen.ID = wrap.Data.ID
	}
	if gen.ID == "" {
		return "", fmt.Errorf("hedra generation id missing: %s", redactBody(b, 200))
	}
	logrus.WithField("generation_id", gen.ID).Info("Hedra avatar generation started")
	return h.pollGeneration(gen.ID)
}

func (h *HedraClient) pollGeneration(id string) (string, error) {
	deadline := time.Now().Add(10 * time.Minute)
	for time.Now().Before(deadline) {
		req, err := http.NewRequest("GET", h.base+"/generations/"+id+"/status", nil)
		if err != nil {
			return "", err
		}
		req.Header.Set("X-API-Key", h.apiKey)
		resp, err := h.client.Do(req)
		if err != nil {
			time.Sleep(5 * time.Second)
			continue
		}
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		var st struct {
			Status   string  `json:"status"`
			Progress float64 `json:"progress"`
			AssetID  string  `json:"asset_id"`
			URL      string  `json:"url"`
			Error    string  `json:"error"`
		}
		_ = json.Unmarshal(b, &st)
		switch st.Status {
		case "complete", "completed", "success":
			if st.URL != "" {
				return st.URL, nil
			}
			if st.AssetID != "" {
				return h.assetURL(st.AssetID)
			}
			return "", fmt.Errorf("hedra complete but no url: %s", redactBody(b, 200))
		case "failed", "error":
			msg := st.Error
			if msg == "" {
				msg = string(b)
			}
			return "", fmt.Errorf("hedra failed: %s", msg)
		}
		time.Sleep(5 * time.Second)
	}
	return "", fmt.Errorf("hedra timeout")
}

func (h *HedraClient) assetURL(assetID string) (string, error) {
	req, err := http.NewRequest("GET", h.base+"/assets/"+assetID, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("X-API-Key", h.apiKey)
	resp, err := h.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	var a struct {
		URL  string `json:"url"`
		Data struct {
			URL string `json:"url"`
		} `json:"data"`
	}
	_ = json.Unmarshal(b, &a)
	u := a.URL
	if u == "" {
		u = a.Data.URL
	}
	if u == "" {
		return "", fmt.Errorf("hedra asset url missing: %s", redactBody(b, 200))
	}
	return u, nil
}
