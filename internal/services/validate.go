package services

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// NormalizeGenerateMode maps UI mode → idea | lyrics | instrumental.
func NormalizeGenerateMode(mode string, instrumental bool) string {
	mode = strings.ToLower(strings.TrimSpace(mode))
	switch mode {
	case "idea", "lyrics", "instrumental":
		return mode
	}
	if instrumental {
		return "instrumental"
	}
	return "idea"
}

func ValidateGenerate(prompt, style, lyrics string, duration int, instrumental bool) error {
	return ValidateGenerateMode("", prompt, style, lyrics, duration, instrumental)
}

func ValidateGenerateMode(mode, prompt, style, lyrics string, duration int, instrumental bool) error {
	mode = NormalizeGenerateMode(mode, instrumental)
	prompt = strings.TrimSpace(prompt)
	lyrics = strings.TrimSpace(lyrics)
	style = strings.TrimSpace(style)

	switch mode {
	case "instrumental":
		if prompt == "" && style == "" {
			return fmt.Errorf("для инструментала нужен prompt или style")
		}
	case "lyrics":
		if lyrics == "" {
			return fmt.Errorf("режим «свой текст»: укажите lyrics")
		}
	default: // idea
		if prompt == "" && lyrics == "" {
			return fmt.Errorf("нужен prompt или lyrics")
		}
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
	return nil
}

// LooksLikeAudio checks magic bytes (MP3/ID3, WAV, M4A, Ogg).
func LooksLikeAudio(data []byte) bool {
	if len(data) < 12 {
		return false
	}
	if string(data[0:3]) == "ID3" {
		return true
	}
	if data[0] == 0xff && data[1]&0xe0 == 0xe0 {
		return true
	}
	if string(data[0:4]) == "RIFF" && string(data[8:12]) == "WAVE" {
		return true
	}
	if string(data[4:8]) == "ftyp" {
		return true
	}
	if string(data[0:4]) == "OggS" {
		return true
	}
	return false
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
