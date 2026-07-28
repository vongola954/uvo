package clients

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"time"
)

// Status probes whether the API key is accepted.
// AceData may not expose a public balance endpoint; we treat 401/403 used_up on a lightweight call.
type AceDataStatus struct {
	Configured bool   `json:"configured"`
	OK         bool   `json:"ok"`
	Code       string `json:"code,omitempty"`
	Message    string `json:"message,omitempty"`
}

func (c *AceDataClient) Status() AceDataStatus {
	st := AceDataStatus{Configured: c.apiKey != ""}
	if !st.Configured {
		st.Code = "not_configured"
		st.Message = "SUNO_API_KEY пуст"
		return st
	}
	// Minimal authenticated request: retrieve with invalid id — auth errors vs not found
	payload, _ := json.Marshal(map[string]interface{}{
		"id":     "00000000-0000-0000-0000-000000000000",
		"action": "retrieve",
	})
	req, err := http.NewRequest("POST", c.tasksURL, bytes.NewReader(payload))
	if err != nil {
		st.Code = "client_error"
		st.Message = err.Error()
		return st
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		st.Code = "network_error"
		st.Message = err.Error()
		return st
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == 401 {
		st.Code = "provider_auth"
		st.Message = "Неверный SUNO_API_KEY"
		return st
	}
	// 403 with used_up on some endpoints
	if err := ParseAceDataHTTPError(resp.StatusCode, body); err != nil {
		if pe, ok := err.(*ProviderError); ok && pe.Code == "provider_balance_empty" {
			st.Code = pe.Code
			st.Message = pe.Message
			return st
		}
	}
	// 404/200/400 with key accepted → OK (key works; balance unknown until generate)
	if resp.StatusCode < 500 {
		st.OK = true
		st.Code = "key_ok"
		st.Message = "Ключ принят. Баланс проверьте на platform.acedata.cloud при ошибке used_up"
		return st
	}
	st.Code = "upstream"
	st.Message = string(body)
	if len(st.Message) > 120 {
		st.Message = st.Message[:120]
	}
	return st
}
