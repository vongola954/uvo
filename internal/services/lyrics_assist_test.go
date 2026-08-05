package services

import (
	"strings"
	"testing"
)

func TestResolveLyricsLLMConfigFreeDefault(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("OPENAI_BASE_URL", "")
	t.Setenv("OPENAI_API_BASE", "")
	t.Setenv("OPENAI_MODEL", "")
	t.Setenv("LYRICS_LLM_PROVIDER", "")
	t.Setenv("LYRICS_ASSIST", "")
	t.Setenv("GEMINI_API_KEY", "")
	t.Setenv("VEO_API_KEY", "")
	t.Setenv("GOOGLE_API_KEY", "")

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
	t.Setenv("GEMINI_API_KEY", "")
	t.Setenv("VEO_API_KEY", "")
	t.Setenv("GOOGLE_API_KEY", "")

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
	t.Setenv("GEMINI_API_KEY", "")
	t.Setenv("VEO_API_KEY", "")
	t.Setenv("GOOGLE_API_KEY", "")

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
}

func TestResolveLyricsLLMConfigPollinationsIgnoresKey(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "pollen-paid-key-with-zero-budget")
	t.Setenv("OPENAI_BASE_URL", "https://text.pollinations.ai/openai")
	t.Setenv("OPENAI_MODEL", "gpt-4o-mini")
	t.Setenv("LYRICS_LLM_PROVIDER", "")
	t.Setenv("LYRICS_ASSIST", "")
	t.Setenv("GEMINI_API_KEY", "")
	t.Setenv("VEO_API_KEY", "")
	t.Setenv("GOOGLE_API_KEY", "")

	cfg := resolveLyricsLLMConfig()
	if cfg.SendAuth || cfg.APIKey != "" {
		t.Fatalf("pollinations must be anonymous, got %+v", cfg)
	}
	if cfg.Model != defaultFreeLLMModel {
		t.Fatalf("model=%s", cfg.Model)
	}
}

func TestLocalLyricsDraft(t *testing.T) {
	text := localLyricsDraft("дождь в городе ночью", "поп")
	if !strings.Contains(text, "[Припев]") || !strings.Contains(text, "дождь") {
		t.Fatalf("bad draft: %s", text)
	}
	if strings.Contains(text, "город молчит") {
		t.Fatal("old fixed template leaked")
	}
}

func TestLocalLyricsDraftVaries(t *testing.T) {
	a := localLyricsDraft("метро и весна", "инди")
	b := localLyricsDraft("метро и весна", "инди")
	if a == b {
		t.Fatal("expected different drafts for repeated calls")
	}
	c := localLyricsDraft("космическая любовь на танцполе", "электронная")
	if strings.Count(a, "[Куплет 1]") != 1 || !strings.Contains(c, "космическая") && !strings.Contains(c, "танцполе") && !strings.Contains(c, "любовь") {
		t.Fatalf("expected idea keywords in draft:\n%s", c)
	}
}

func TestLyricsAssistDraftFallsBackLocal(t *testing.T) {
	t.Setenv("LYRICS_LLM_PROVIDER", "local")
	t.Setenv("LYRICS_ASSIST", "true")
	text, err := LyricsAssistDraft("u1", "метро и весна", "инди", nil)
	if err != nil || !strings.Contains(text, "[Куплет 1]") {
		t.Fatalf("err=%v text=%q", err, text)
	}
}
