package services

import (
	"strings"
	"testing"
)

func TestInferStyleFromTextRap(t *testing.T) {
	s := InferStyleFromText("улицы и рифмы", "[Куплет]\nмой рэп про город, trap beat в голове")
	if !strings.Contains(strings.ToLower(s), "хип") && !strings.Contains(strings.ToLower(s), "r&b") {
		t.Fatalf("got %q", s)
	}
}

func TestInferStyleFromTextLovePop(t *testing.T) {
	s := InferStyleFromText("любовь под дождём", "сердце бьётся, припев про нас")
	if !strings.Contains(strings.ToLower(s), "поп") {
		t.Fatalf("got %q", s)
	}
}

func TestInferStyleFromTextEmpty(t *testing.T) {
	s := InferStyleFromText("", "")
	if s == "" {
		t.Fatal("expected default")
	}
}
