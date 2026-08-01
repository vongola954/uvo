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
	c5, p5, _, ok5 := PackByID("pack5")
	if !ok5 || c5 != 5 || p5 != 99 {
		t.Fatalf("pack5: %d %d %v", c5, p5, ok5)
	}
	if _, _, _, ok := PackByID("nope"); ok {
		t.Fatal("unknown pack")
	}
	packs := PacksPublic()
	if len(packs) < 1 || packs[0].RubPerSong <= 0 {
		t.Fatal("packs need rub_per_song")
	}
}
