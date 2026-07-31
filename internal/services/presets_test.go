package services

import "testing"

func TestViralPresetsNonEmpty(t *testing.T) {
	if len(ViralPresets) < 10 {
		t.Fatalf("want >=10 presets, got %d", len(ViralPresets))
	}
	p, ok := PresetByID("night-drive")
	if !ok || p.Prompt == "" {
		t.Fatal("night-drive missing")
	}
}

func TestPackByID(t *testing.T) {
	c, price, name, ok := PackByID("pack10")
	if !ok || c != 10 || price != 199 || name == "" {
		t.Fatalf("pack10: %d %d %q %v", c, price, name, ok)
	}
	if _, _, _, ok := PackByID("nope"); ok {
		t.Fatal("unknown pack")
	}
}
