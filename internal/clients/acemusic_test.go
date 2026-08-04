package clients

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"uvo/internal/config"
)

func TestParseAceMusicCompletion(t *testing.T) {
	raw := []byte(`{
	  "id":"chatcmpl-1",
	  "choices":[{
	    "message":{
	      "role":"assistant",
	      "content":"## Metadata\n**Caption:** Night Drive\n\n## Lyrics\n[Verse] go\n",
	      "audio":[{"type":"audio_url","audio_url":{"url":"data:audio/mpeg;base64,AAA"}}]
	    }
	  }]
	}`)
	clips, title, lyric, err := parseAceMusicCompletion(raw)
	if err != nil {
		t.Fatal(err)
	}
	if title != "Night Drive" {
		t.Fatalf("title %q", title)
	}
	if !strings.Contains(lyric, "[Verse]") {
		t.Fatalf("lyric %q", lyric)
	}
	if len(clips) != 1 || !strings.HasPrefix(clips[0].AudioURL, "data:audio") {
		t.Fatalf("clips %+v", clips)
	}
}

func TestAceMusicGenerateAll(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			http.NotFound(w, r)
			return
		}
		if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
			http.Error(w, "auth", 401)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"id": "x",
			"choices": []map[string]interface{}{
				{"message": map[string]interface{}{
					"content": "ok",
					"audio": []map[string]interface{}{
						{"audio_url": map[string]string{"url": "data:audio/mpeg;base64,QQ=="}},
					},
				}},
			},
		})
	}))
	defer srv.Close()

	c := NewAceMusicClient(&config.Config{
		AceMusicAPIKey:  "test-key",
		AceMusicBaseURL: srv.URL,
		AceMusicModel:   "acemusic/acestep-v1.5-turbo",
		AceMusicTimeout: 10,
	})
	clips, err := c.GenerateAll(&GenerateRequest{Prompt: "jazz", Duration: 30})
	if err != nil {
		t.Fatal(err)
	}
	if len(clips) != 1 {
		t.Fatalf("want 1 clip, got %d", len(clips))
	}
}

func TestParseAceMusicHTTPErrorAuth(t *testing.T) {
	err := ParseAceMusicHTTPError(401, []byte(`{"error":{"message":"bad key"}}`))
	pe := AsProviderError(err)
	if pe == nil || pe.Code != "provider_auth" {
		t.Fatalf("%v", err)
	}
}

func TestBuildAceMusicContent(t *testing.T) {
	got := buildAceMusicContent(&GenerateRequest{Prompt: "pop", Lyric: "hello", Style: "dance"})
	if !strings.Contains(got, "<prompt>") || !strings.Contains(got, "hello") {
		t.Fatalf("%q", got)
	}
	inst := buildAceMusicContent(&GenerateRequest{Prompt: "edm", Instrumental: true})
	if !strings.Contains(inst, "[inst]") {
		t.Fatalf("%q", inst)
	}
}
