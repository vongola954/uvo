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

func TestResolveLyricsLLMConfigPollinationsIgnoresKey(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "pollen-paid-key-with-zero-budget")
	t.Setenv("OPENAI_BASE_URL", "https://text.pollinations.ai/openai")
	t.Setenv("OPENAI_MODEL", "gpt-4o-mini")
	t.Setenv("LYRICS_LLM_PROVIDER", "")
	t.Setenv("LYRICS_ASSIST", "")

	cfg := resolveLyricsLLMConfig()
	if cfg.SendAuth || cfg.APIKey != "" {
		t.Fatalf("pollinations must be anonymous, got %+v", cfg)
	}
	if cfg.Model != defaultFreeLLMModel {
		t.Fatalf("model=%s", cfg.Model)
	}
}

func TestFinalizeLLMAuthStripsPollinationsBearer(t *testing.T) {
	cfg := finalizeLLMAuth(lyricsLLMConfig{
		BaseURL: defaultFreeLLMBase, APIKey: "x", Model: "gpt-4o-mini", Provider: "custom", SendAuth: true,
	})
	if cfg.SendAuth || cfg.APIKey != "" || cfg.Provider != "pollinations" {
		t.Fatalf("got %+v", cfg)
	}
}
