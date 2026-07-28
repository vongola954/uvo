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

type StemResult struct {
	Vocals       *GenerateResponse
	Instrumental *GenerateResponse
}

type TimingWord struct {
	Word   string  `json:"word"`
	StartS float64 `json:"start_s"`
	EndS   float64 `json:"end_s"`
}

type TimingResult struct {
	Words []TimingWord `json:"aligned_words"`
}

// Stems separates vocals / instrumental for a Suno audio_id.
func (c *AceDataClient) Stems(audioID string) (*StemResult, error) {
	if audioID == "" {
		return nil, fmt.Errorf("audio_id required")
	}
	payload := map[string]interface{}{
		"action":   "stems",
		"audio_id": audioID,
		"async":    true,
		"model":    c.model,
	}
	items, taskID, err := c.postAudiosAndPollAll(payload)
	if err != nil {
		return nil, err
	}
	out := &StemResult{}
	for i := range items {
		item := items[i]
		gr := itemToGenerateResponse(item, taskID)
		title := strings.ToLower(item.Title)
		switch {
		case strings.Contains(title, "instrumental") || strings.Contains(title, "accompan"):
			out.Instrumental = gr
		case strings.Contains(title, "vocal"):
			out.Vocals = gr
		default:
			if out.Vocals == nil {
				out.Vocals = gr
			} else if out.Instrumental == nil {
				out.Instrumental = gr
			}
		}
	}
	if out.Instrumental == nil && out.Vocals == nil {
		return nil, fmt.Errorf("stems returned no usable tracks")
	}
	// If only one track classified, swap heuristics by order (docs: vocals first, instrumental second)
	if out.Instrumental == nil && len(items) >= 2 {
		out.Instrumental = itemToGenerateResponse(items[1], taskID)
	}
	if out.Vocals == nil && len(items) >= 1 {
		out.Vocals = itemToGenerateResponse(items[0], taskID)
	}
	return out, nil
}

// GetMP4 returns official Suno visualizer video for audio_id.
func (c *AceDataClient) GetMP4(audioID string) (videoURL string, err error) {
	if audioID == "" {
		return "", fmt.Errorf("audio_id required")
	}
	url := "https://api.acedata.cloud/suno/mp4"
	body, err := c.postJSON(url, map[string]interface{}{"audio_id": audioID})
	if err != nil {
		return "", err
	}
	var resp struct {
		Success bool `json:"success"`
		Data    struct {
			VideoURL string `json:"video_url"`
		} `json:"data"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", err
	}
	if resp.Data.VideoURL == "" {
		if resp.Error != "" {
			return "", fmt.Errorf("mp4: %s", resp.Error)
		}
		return "", fmt.Errorf("mp4: empty video_url: %s", redactBody(body, 200))
	}
	return resp.Data.VideoURL, nil
}

// GetTiming returns lyric word alignment for karaoke.
func (c *AceDataClient) GetTiming(audioID string) (*TimingResult, error) {
	if audioID == "" {
		return nil, fmt.Errorf("audio_id required")
	}
	url := "https://api.acedata.cloud/suno/timing"
	body, err := c.postJSON(url, map[string]interface{}{"audio_id": audioID})
	if err != nil {
		return nil, err
	}
	var resp struct {
		Success bool `json:"success"`
		Data    struct {
			AlignedWords []TimingWord `json:"aligned_words"`
		} `json:"data"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	if resp.Error != "" {
		return nil, fmt.Errorf("timing: %s", resp.Error)
	}
	return &TimingResult{Words: resp.Data.AlignedWords}, nil
}

func (c *AceDataClient) postAudiosAndPollAll(payload map[string]interface{}) ([]aceAudioItem, string, error) {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, "", err
	}
	logrus.WithField("payload", redactBody(jsonData, 256)).Debug("AceData audios multi request")
	httpReq, err := http.NewRequest("POST", c.baseURL, bytes.NewReader(jsonData))
	if err != nil {
		return nil, "", err
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, "", ParseAceDataHTTPError(resp.StatusCode, body)
	}
	var createResp aceCreateResponse
	if err := json.Unmarshal(body, &createResp); err != nil {
		return nil, "", err
	}
	// Inline data
	var inline aceTaskResponse
	if json.Unmarshal(body, &inline) == nil && len(inline.Data) > 0 && allSucceeded(inline.Data) {
		return inline.Data, createResp.TaskID, nil
	}
	if createResp.TaskID == "" {
		return nil, "", fmt.Errorf("no task_id: %s", redactBody(body, 200))
	}
	return c.pollTaskAll(createResp.TaskID)
}

func allSucceeded(items []aceAudioItem) bool {
	ok := false
	for _, it := range items {
		if it.AudioURL == "" {
			return false
		}
		st := strings.ToLower(it.State)
		if st == "" || st == "succeeded" || st == "completed" || st == "success" {
			ok = true
			continue
		}
		if st == "pending" || st == "processing" || st == "running" {
			return false
		}
	}
	return ok
}

func (c *AceDataClient) pollTaskAll(taskID string) ([]aceAudioItem, string, error) {
	start := time.Now()
	for {
		if time.Since(start) > time.Duration(c.maxWait)*time.Second {
			return nil, "", fmt.Errorf("task %s timeout", taskID)
		}
		payload := map[string]interface{}{"id": taskID, "action": "retrieve"}
		taskData, _ := json.Marshal(payload)
		taskReq, err := http.NewRequest("POST", c.tasksURL, bytes.NewReader(taskData))
		if err != nil {
			return nil, "", err
		}
		taskReq.Header.Set("Authorization", "Bearer "+c.apiKey)
		taskReq.Header.Set("Content-Type", "application/json")
		taskReq.Header.Set("Accept", "application/json")
		taskResp, err := c.client.Do(taskReq)
		if err != nil {
			time.Sleep(time.Duration(c.pollInt) * time.Second)
			continue
		}
		body, err := io.ReadAll(taskResp.Body)
		taskResp.Body.Close()
		if err != nil {
			time.Sleep(time.Duration(c.pollInt) * time.Second)
			continue
		}
		var taskResult aceTaskResponse
		if err := json.Unmarshal(body, &taskResult); err != nil {
			return nil, "", err
		}
		if taskResult.Error != "" {
			return nil, "", fmt.Errorf("task failed: %s", taskResult.Error)
		}
		if len(taskResult.Data) > 0 && allSucceeded(taskResult.Data) {
			return taskResult.Data, taskID, nil
		}
		time.Sleep(time.Duration(c.pollInt) * time.Second)
	}
}
