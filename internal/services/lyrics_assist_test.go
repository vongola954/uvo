package services

import (
	"testing"
)

func TestResolveLyricsLLMConfigFreeDefault(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("OPENAI_BASE_URL", "")
	t.Setenv("OPENAI_API_BASE", "")
	t.Setenv("OPENAI_MODEL", "")
	t.Setenv("LYRICS_LLM_PROVIDER", "")
	t.Setenv("LYRICS_ASSIST", "")

	cfg := resolveLyricsLLMConfig()
	if cfg.Provider != "pollinations" || cfg.BaseURL != defaultFreeLLMBase {
		t.Fatalf("got %+v", cfg)
	}
	if cfg.SendAuth {
		t.Fatal("pollinations anonymous must not send Authorization")
	}
	if !LyricsAssistEnabled() {
		t.Fatal("expected enabled")
	}
}

func TestResolveLyricsLLMConfigOpenAI(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "sk-test-real")
	t.Setenv("OPENAI_BASE_URL", "")
	t.Setenv("OPENAI_MODEL", "")
	t.Setenv("LYRICS_LLM_PROVIDER", "")
	t.Setenv("LYRICS_ASSIST", "")

	cfg := resolveLyricsLLMConfig()
	if cfg.Provider != "openai" || cfg.BaseURL != "https://api.openai.com/v1" || !cfg.SendAuth {
		t.Fatalf("got %+v", cfg)
	}
}

func TestResolveLyricsLLMConfigPlaceholderUsesFree(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "not-needed")
	t.Setenv("OPENAI_BASE_URL", "")
	t.Setenv("LYRICS_LLM_PROVIDER", "")
	t.Setenv("LYRICS_ASSIST", "")

	cfg := resolveLyricsLLMConfig()
	if cfg.Provider != "pollinations" || cfg.SendAuth {
		t.Fatalf("got %+v", cfg)
	}
}

func TestResolveLyricsLLMConfigDisabled(t *testing.T) {
	t.Setenv("LYRICS_ASSIST", "false")
	t.Setenv("OPENAI_API_KEY", "sk-test")
	if LyricsAssistEnabled() {
		t.Fatal("expected disabled")
	}
	if resolveLyricsLLMConfig().BaseURL != "" {
		t.Fatal("expected empty config")
	}
}

func TestIsPlaceholderAPIKey(t *testing.T) {
	if !isPlaceholderAPIKey("not-needed") || isPlaceholderAPIKey("sk-abc") {
		t.Fatal("placeholder detection failed")
	}
}
