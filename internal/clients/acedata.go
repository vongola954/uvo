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

	"github.com/sirupsen/logrus"
	"uvo/internal/config"
)

type AceDataClient struct {
	apiKey    string
	baseURL   string
	tasksURL  string
	voicesURL string
	uploadURL string
	async     bool
	pollInt   int
	maxWait   int
	model     string
	client    *http.Client
}

func NewAceDataClient(cfg *config.Config) *AceDataClient {
	return &AceDataClient{
		apiKey:    cfg.SunoAPIKey,
		baseURL:   cfg.AceDataSunoURL,
		tasksURL:  cfg.AceDataTasksURL,
		voicesURL: cfg.AceDataVoicesURL,
		uploadURL: cfg.AceDataUploadURL,
		async:     cfg.AceDataAsync,
		pollInt:   cfg.AceDataPollInterval,
		maxWait:   cfg.AceDataMaxWait,
		model:     cfg.SunoModel,
		client:    HTTPClient(120 * time.Second),
	}
}

type GenerateRequest struct {
	Custom       bool   `json:"custom"`
	Prompt       string `json:"prompt,omitempty"`
	Lyric        string `json:"lyric,omitempty"`
	Style        string `json:"style,omitempty"`
	Title        string `json:"title,omitempty"`
	Instrumental bool   `json:"instrumental,omitempty"`
	Model        string `json:"model,omitempty"`
	PersonaID    string `json:"persona_id,omitempty"`
	Duration     int    `json:"duration,omitempty"` // seconds; used by AceMusic
}

type GenerateResponse struct {
	AudioURL   string
	VideoURL   string
	AudioID    string
	TaskID     string
	Title      string
	Duration   float64
	Lyric      string
	AudioBytes []byte // optional in-memory audio (AceMusic); skips HTTP download
}

// Real AceData response structures
type aceCreateResponse struct {
	Success bool   `json:"success"`
	TaskID  string `json:"task_id"`
	TraceID string `json:"trace_id"`
	Message string `json:"message"`
}

type aceAudioItem struct {
	ID       string  `json:"id"`
	Title    string  `json:"title"`
	Style    string  `json:"style"`
	Lyric    string  `json:"lyric"`
	Duration float64 `json:"duration"`
	AudioURL string  `json:"audio_url"`
	ImageURL string  `json:"image_url"`
	VideoURL string  `json:"video_url"`
	State    string  `json:"state"` // succeeded | failed | pending | processing
}

type aceTaskResponse struct {
	Success bool           `json:"success"`
	TaskID  string         `json:"task_id"`
	Data    []aceAudioItem `json:"data"`
	Message string         `json:"message"`
	Error   string         `json:"error"`
}

func (c *AceDataClient) Generate(req *GenerateRequest) (*GenerateResponse, error) {
	clips, err := c.GenerateAll(req)
	if err != nil {
		return nil, err
	}
	return clips[0], nil
}

// GenerateAll returns 1–2 clips (2 when DUAL_OUTPUT=true and AceData provides them).
func (c *AceDataClient) GenerateAll(req *GenerateRequest) ([]*GenerateResponse, error) {
	if !c.async {
		return nil, fmt.Errorf("sync mode not supported")
	}
	return c.generateAsyncAll(req)
}

func (c *AceDataClient) generateAsyncAll(req *GenerateRequest) ([]*GenerateResponse, error) {
	model := c.model
	if req.Model != "" {
		model = req.Model
	}

	payload := map[string]interface{}{
		"action": "generate",
		"model":  model,
		"async":  true,
	}

	if req.Custom && req.Lyric != "" {
		payload["custom"] = true
		payload["lyric"] = req.Lyric
		if req.Style != "" {
			payload["style"] = req.Style
		}
		if req.Title != "" {
			payload["title"] = req.Title
		}
	} else {
		payload["custom"] = false
		payload["prompt"] = req.Prompt
		if req.Style != "" {
			payload["style"] = req.Style
		}
	}

	if req.Instrumental {
		payload["instrumental"] = true
	}
	if req.PersonaID != "" {
		payload["persona_id"] = req.PersonaID
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	logrus.WithField("payload", redactBody(jsonData, 256)).Debug("AceData create request")

	httpReq, err := http.NewRequest("POST", c.baseURL, bytes.NewReader(jsonData))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("acedata request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		logrus.WithFields(logrus.Fields{
			"status": resp.StatusCode,
			"body":   redactBody(body, 256),
		}).Warn("AceData create failed")
		return nil, ParseAceDataHTTPError(resp.StatusCode, body)
	}
	logrus.WithFields(logrus.Fields{
		"status": resp.StatusCode,
		"body":   redactBody(body, 256),
	}).Info("AceData create ok")

	var createResp aceCreateResponse
	if err := json.Unmarshal(body, &createResp); err != nil {
		return nil, fmt.Errorf("decode create response: %w, body: %s", err, string(body))
	}

	if createResp.TaskID == "" {
		return nil, fmt.Errorf("acedata did not return task_id, body: %s", string(body))
	}

	want := 1
	if strings.EqualFold(strings.TrimSpace(os.Getenv("DUAL_OUTPUT")), "true") {
		want = 2
	}
	return c.pollTaskClips(createResp.TaskID, want)
}

// pollTask waits for the first succeeded clip (used by voice/cover helpers).
func (c *AceDataClient) pollTask(taskID string) (*GenerateResponse, error) {
	clips, err := c.pollTaskClips(taskID, 1)
	if err != nil {
		return nil, err
	}
	return clips[0], nil
}

// pollTaskClips waits for up to want succeeded clips (1–2).
func (c *AceDataClient) pollTaskClips(taskID string, want int) ([]*GenerateResponse, error) {
	if want < 1 {
		want = 1
	}
	if want > 2 {
		want = 2
	}
	start := time.Now()
	attempt := 0
	var lastClips []*GenerateResponse

	for {
		attempt++
		if time.Since(start) > time.Duration(c.maxWait)*time.Second {
			if len(lastClips) >= 1 {
				return lastClips, nil
			}
			return nil, fmt.Errorf("task %s timeout after %d seconds (%d attempts)", taskID, c.maxWait, attempt)
		}

		payload := map[string]interface{}{
			"id":     taskID,
			"action": "retrieve",
		}
		taskData, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("marshal poll payload: %w", err)
		}

		taskReq, err := http.NewRequest("POST", c.tasksURL, bytes.NewReader(taskData))
		if err != nil {
			return nil, fmt.Errorf("create poll request: %w", err)
		}
		taskReq.Header.Set("Authorization", "Bearer "+c.apiKey)
		taskReq.Header.Set("Content-Type", "application/json")
		taskReq.Header.Set("Accept", "application/json")

		taskResp, err := c.client.Do(taskReq)
		if err != nil {
			logrus.WithError(err).Warn("poll request failed, retrying")
			time.Sleep(time.Duration(c.pollInt) * time.Second)
			continue
		}

		body, err := io.ReadAll(taskResp.Body)
		taskResp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("read poll body: %w", err)
		}

		logrus.WithFields(logrus.Fields{
			"task_id": taskID,
			"attempt": attempt,
			"status":  taskResp.StatusCode,
			"body":    redactBody(body, 256),
		}).Debug("AceData poll response")

		if taskResp.StatusCode != http.StatusOK {
			logrus.WithField("body", redactBody(body, 128)).Warn("poll non-200, retrying")
			time.Sleep(time.Duration(c.pollInt) * time.Second)
			continue
		}

		var taskResult aceTaskResponse
		if err := json.Unmarshal(body, &taskResult); err != nil {
			return nil, fmt.Errorf("decode poll response: %w, body: %s", err, string(body))
		}

		if taskResult.Error != "" {
			return nil, fmt.Errorf("task failed: %s", taskResult.Error)
		}

		var clips []*GenerateResponse
		allFailed := len(taskResult.Data) > 0
		for _, item := range taskResult.Data {
			switch item.State {
			case "succeeded", "completed", "success":
				allFailed = false
				if item.AudioURL != "" {
					clips = append(clips, itemToGenerateResponse(item, taskID))
				}
			case "failed", "error":
				// keep scanning siblings
			default:
				allFailed = false
			}
		}
		// Fallback: success flag + audio without state
		if len(clips) == 0 && taskResult.Success {
			for _, item := range taskResult.Data {
				if item.AudioURL != "" {
					clips = append(clips, itemToGenerateResponse(item, taskID))
				}
			}
		}
		if len(clips) > 0 {
			lastClips = clips
		}
		if len(clips) >= want {
			if len(clips) > want {
				clips = clips[:want]
			}
			return clips, nil
		}
		if len(clips) >= 1 && allFailed {
			return clips, nil
		}
		if allFailed && len(clips) == 0 {
			return nil, fmt.Errorf("task failed with state=failed, response: %s", string(body))
		}

		time.Sleep(time.Duration(c.pollInt) * time.Second)
	}
}

func itemToGenerateResponse(item aceAudioItem, taskID string) *GenerateResponse {
	return &GenerateResponse{
		AudioURL: item.AudioURL,
		VideoURL: item.VideoURL,
		AudioID:  item.ID,
		TaskID:   taskID,
		Title:    item.Title,
		Duration: item.Duration,
		Lyric:    item.Lyric,
	}
}
