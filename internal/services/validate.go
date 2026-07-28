package services

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

func ValidateGenerate(prompt, style, lyrics string, duration int, instrumental bool) error {
	prompt = strings.TrimSpace(prompt)
	if !instrumental && prompt == "" && strings.TrimSpace(lyrics) == "" {
		return fmt.Errorf("нужен prompt или lyrics")
	}
	if utf8.RuneCountInString(prompt) > 500 {
		return fmt.Errorf("prompt максимум 500 символов")
	}
	if utf8.RuneCountInString(lyrics) > 5000 {
		return fmt.Errorf("lyrics максимум 5000 символов")
	}
	if utf8.RuneCountInString(style) > 1000 {
		return fmt.Errorf("style максимум 1000 символов")
	}
	if duration < 0 {
		return fmt.Errorf("duration не может быть отрицательным")
	}
	if duration > 480 {
		return fmt.Errorf("duration максимум 480 секунд")
	}
	return nil
}

func ValidateVoiceUpload(name string, size int64, contentType string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("укажите name голоса")
	}
	if utf8.RuneCountInString(name) > 80 {
		return fmt.Errorf("name максимум 80 символов")
	}
	if size < 1000 {
		return fmt.Errorf("файл слишком маленький (нужно ~10–30 сек аудио)")
	}
	if size > 15*1024*1024 {
		return fmt.Errorf("файл больше 15 MB")
	}
	ct := strings.ToLower(contentType)
	if ct != "" && !strings.Contains(ct, "audio") && !strings.Contains(ct, "octet-stream") &&
		!strings.HasSuffix(strings.ToLower(name), ".mp3") &&
		!strings.HasSuffix(strings.ToLower(name), ".wav") &&
		!strings.HasSuffix(strings.ToLower(name), ".m4a") &&
		!strings.HasSuffix(strings.ToLower(name), ".ogg") {
		// content-type often wrong from browsers; size checks are primary
	}
	return nil
}

func ValidateTTS(voiceID, text string) error {
	if strings.TrimSpace(voiceID) == "" {
		return fmt.Errorf("voice_id обязателен")
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return fmt.Errorf("text обязателен")
	}
	if utf8.RuneCountInString(text) > 2500 {
		return fmt.Errorf("text максимум 2500 символов")
	}
	return nil
}
