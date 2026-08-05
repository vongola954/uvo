package services

import (
	"errors"
	"testing"

	"uvo/internal/clients"
)

type stubMusicGen struct {
	name  string
	err   error
	clips []*clients.GenerateResponse
}

func (s *stubMusicGen) GenerateAll(req *clients.GenerateRequest) ([]*clients.GenerateResponse, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.clips, nil
}

func TestGenerateClipsFallbackToAceMusic(t *testing.T) {
	ace := &stubMusicGen{name: "acedata", err: &clients.ProviderError{
		Code: "provider_balance_empty", Message: "empty", Status: 503,
	}}
	music := &stubMusicGen{name: "acemusic", clips: []*clients.GenerateResponse{
		{AudioURL: "data:audio/mpeg;base64,QQ==", Title: "ok"},
	}}
	svc := &GenerationService{
		aceClient:     ace,
		aceMusic:      music,
		musicProvider: "auto",
	}
	clips, used, err := svc.generateClips(&clients.GenerateRequest{Prompt: "x"}, "auto")
	if err != nil {
		t.Fatal(err)
	}
	if used != "acemusic" || len(clips) != 1 {
		t.Fatalf("used=%s clips=%d", used, len(clips))
	}
}

func TestGenerateClipsAceMusicOnly(t *testing.T) {
	music := &stubMusicGen{clips: []*clients.GenerateResponse{{AudioURL: "data:audio/mpeg;base64,QQ=="}}}
	svc := &GenerationService{aceMusic: music, musicProvider: "acemusic"}
	_, used, err := svc.generateClips(&clients.GenerateRequest{Prompt: "x"}, "")
	if err != nil || used != "acemusic" {
		t.Fatalf("used=%s err=%v", used, err)
	}
}

func TestShouldFallbackMusic(t *testing.T) {
	if !shouldFallbackMusic(&clients.ProviderError{Code: "provider_balance_empty"}) {
		t.Fatal("expected fallback")
	}
	if !shouldFallbackMusic(&clients.ProviderError{Code: "provider_timeout", Status: 504}) {
		t.Fatal("expected timeout fallback")
	}
	if !shouldFallbackMusic(errors.New("AceMusic HTTP 504: Gateway time-out")) {
		t.Fatal("expected 504 string fallback")
	}
	if shouldFallbackMusic(errors.New("random")) {
		t.Fatal("unexpected fallback")
	}
}

func TestGenerateClipsAceMusicTimeoutFallsBackAceData(t *testing.T) {
	music := &stubMusicGen{err: &clients.ProviderError{Code: "provider_timeout", Status: 504, Message: "cf"}}
	ace := &stubMusicGen{clips: []*clients.GenerateResponse{{AudioURL: "data:audio/mpeg;base64,QQ==", Title: "ok"}}}
	svc := &GenerationService{aceClient: ace, aceMusic: music, musicProvider: "acemusic"}
	clips, used, err := svc.generateClips(&clients.GenerateRequest{Prompt: "x"}, "acemusic")
	if err != nil || used != "acedata" || len(clips) != 1 {
		t.Fatalf("used=%s err=%v clips=%d", used, err, len(clips))
	}
}
