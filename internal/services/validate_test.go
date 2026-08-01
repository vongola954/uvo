package services

import "testing"

func TestValidateGenerate(t *testing.T) {
	if err := ValidateGenerate("", "", "", 180, false); err == nil {
		t.Fatal("expected error")
	}
	if err := ValidateGenerate("ok prompt", "Pop", "", 180, false); err != nil {
		t.Fatal(err)
	}
	if err := ValidateGenerate("x", "", "", 9999, false); err == nil {
		t.Fatal("duration")
	}
}

func TestValidateGenerateMode(t *testing.T) {
	if err := ValidateGenerateMode("lyrics", "", "Pop", "", 180, false); err == nil {
		t.Fatal("lyrics mode needs lyrics")
	}
	if err := ValidateGenerateMode("lyrics", "", "Pop", "verse one", 180, false); err != nil {
		t.Fatal(err)
	}
	if err := ValidateGenerateMode("instrumental", "", "", "", 180, true); err == nil {
		t.Fatal("instrumental needs prompt or style")
	}
	if err := ValidateGenerateMode("instrumental", "lofi chill", "", "", 180, true); err != nil {
		t.Fatal(err)
	}
	if NormalizeGenerateMode("", true) != "instrumental" {
		t.Fatal("normalize instrumental")
	}
}

func TestLooksLikeAudio(t *testing.T) {
	if LooksLikeAudio([]byte("not audio!!")) {
		t.Fatal("junk")
	}
	id3 := append([]byte("ID3"), make([]byte, 20)...)
	if !LooksLikeAudio(id3) {
		t.Fatal("ID3")
	}
	wav := make([]byte, 12)
	copy(wav[0:4], "RIFF")
	copy(wav[8:12], "WAVE")
	if !LooksLikeAudio(wav) {
		t.Fatal("WAV")
	}
	mp3 := []byte{0xff, 0xfb, 0x90, 0x00, 0, 0, 0, 0, 0, 0, 0, 0}
	if !LooksLikeAudio(mp3) {
		t.Fatal("mp3 frame")
	}
}
