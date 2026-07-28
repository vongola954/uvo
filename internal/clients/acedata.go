package clients

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
	"uvo/internal/config"
)

type AceDataClient struct {
	apiKey   string
	baseURL  string
	tasksURL string
	async    bool
	pollInt  int
	maxWait  int
	model    string
	client   *http.Client
}

func NewAceDataClient(cfg *config.Config) *AceDataClient {
	return &AceDataClient{
		apiKey:   cfg.SunoAPIKey,
		baseURL:  cfg.AceDataSunoURL,
		tasksURL: cfg.AceDataTasksURL,
		async:    cfg.AceDataAsync,
		pollInt:  cfg.AceDataPollInterval,
		maxWait:  cfg.AceDataMaxWait,
		model:    cfg.SunoModel,
		client:   &http.Client{Timeout: 120 * time.Second},
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
}

type GenerateResponse struct {
	AudioURL string
	TaskID   string
	Title    string
	Duration float64
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
	if !c.async {
		return nil, fmt.Errorf("sync mode not supported")
	}
	return c.generateAsync(req)
}

func (c *AceDataClient) generateAsync(req *GenerateRequest) (*GenerateResponse, error) {
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

	logrus.WithFields(logrus.Fields{
		"status": resp.StatusCode,
		"body":   redactBody(body, 256),
	}).Debug("AceData create response")

	if resp.StatusCode != http.StatusOK {
		return nil, ParseAceDataHTTPError(resp.StatusCode, body)
	}

	var createResp aceCreateResponse
	if err := json.Unmarshal(body, &createResp); err != nil {
		return nil, fmt.Errorf("decode create response: %w, body: %s", err, string(body))
	}

	if createResp.TaskID == "" {
		return nil, fmt.Errorf("acedata did not return task_id, body: %s", string(body))
	}

	return c.pollTask(createResp.TaskID)
}

func (c *AceDataClient) pollTask(taskID string) (*GenerateResponse, error) {
	start := time.Now()
	attempt := 0

	for {
		attempt++
		if time.Since(start) > time.Duration(c.maxWait)*time.Second {
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

		if len(taskResult.Data) > 0 {
			item := taskResult.Data[0]

			switch item.State {
			case "succeeded", "completed", "success":
				if item.AudioURL == "" {
					return nil, fmt.Errorf("acedata returned empty audio_url, full response: %s", string(body))
				}
				return &GenerateResponse{
					AudioURL: item.AudioURL,
					TaskID:   taskID,
					Title:    item.Title,
					Duration: item.Duration,
				}, nil

			case "failed", "error":
				return nil, fmt.Errorf("task failed with state=%s, response: %s", item.State, string(body))
			}
		}

		// Fallback: success + data with audio_url but without explicit state
		if taskResult.Success && len(taskResult.Data) > 0 && taskResult.Data[0].AudioURL != "" {
			item := taskResult.Data[0]
			return &GenerateResponse{
				AudioURL: item.AudioURL,
				TaskID:   taskID,
				Title:    item.Title,
				Duration: item.Duration,
			}, nil
		}

		time.Sleep(time.Duration(c.pollInt) * time.Second)
	}
}
