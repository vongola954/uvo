package services

import "testing"

func TestLoginCodeConsumeOnce(t *testing.T) {
	s := NewLoginCodeStore()
	code, err := s.Issue("user-1", 0)
	if err != nil || code == "" {
		t.Fatal(err)
	}
	uid, ok := s.Consume(code)
	if !ok || uid != "user-1" {
		t.Fatalf("got %q %v", uid, ok)
	}
	if _, ok := s.Consume(code); ok {
		t.Fatal("code must be single-use")
	}
}
