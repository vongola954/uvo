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
