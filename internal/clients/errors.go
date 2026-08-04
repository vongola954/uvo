package clients

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

type ProviderError struct {
	Code    string
	Message string
	Status  int
}

func (e *ProviderError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func ParseAceDataHTTPError(status int, body []byte) error {
	var payload struct {
		Error struct {
			Message string `json:"message"`
			Code    string `json:"code"`
		} `json:"error"`
		Message string `json:"message"`
	}
	_ = json.Unmarshal(body, &payload)
	msg := payload.Error.Message
	if msg == "" {
		msg = payload.Message
	}
	if msg == "" {
		msg = string(body)
	}
	code := payload.Error.Code
	lower := strings.ToLower(msg + " " + code)

	switch {
	case status == 403 && (code == "used_up" || strings.Contains(lower, "balance") ||
		strings.Contains(lower, "not sufficient") || strings.Contains(lower, "used_up")):
		return &ProviderError{
			Code:    "provider_balance_empty",
			Message: "Баланс AceData/Suno исчерпан. Пополните на https://platform.acedata.cloud",
			Status:  503,
		}
	case status == 401 || status == 403:
		return &ProviderError{
			Code:    "provider_auth",
			Message: "Ошибка авторизации AceData (проверьте SUNO_API_KEY)",
			Status:  502,
		}
	case status == 429:
		return &ProviderError{Code: "provider_rate_limit", Message: "AceData rate limit", Status: 429}
	default:
		return &ProviderError{
			Code:    "provider_error",
			Message: fmt.Sprintf("AceData HTTP %d: %s", status, truncate(msg, 200)),
			Status:  502,
		}
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func AsProviderError(err error) *ProviderError {
	if err == nil {
		return nil
	}
	var pe *ProviderError
	if errors.As(err, &pe) {
		return pe
	}
	msg := err.Error()
	lower := strings.ToLower(msg)
	switch {
	case strings.Contains(msg, "provider_balance_empty") || strings.Contains(msg, "Баланс AceData") ||
		strings.Contains(lower, "used_up") || strings.Contains(lower, "not sufficient"):
		return &ProviderError{
			Code:    "provider_balance_empty",
			Message: "Баланс AceData/Suno исчерпан. Пополните на https://platform.acedata.cloud",
			Status:  503,
		}
	case strings.Contains(lower, "timeout after"):
		return &ProviderError{
			Code:    "provider_timeout",
			Message: "Suno не вернул трек вовремя. Попробуйте ещё раз или короткий промпт.",
			Status:  504,
		}
	case strings.Contains(lower, "task failed"):
		return &ProviderError{
			Code:    "provider_error",
			Message: "Suno отклонил задачу. Измените текст/стиль и попробуйте снова.",
			Status:  502,
		}
	case strings.Contains(lower, "download failed") || strings.Contains(lower, "host not allowlisted"):
		return &ProviderError{
			Code:    "provider_error",
			Message: "Не удалось скачать готовый трек. Попробуйте позже.",
			Status:  502,
		}
	}
	return nil
}
