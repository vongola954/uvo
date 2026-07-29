package services

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

type loginEntry struct {
	UserID string
	Exp    time.Time
}

// LoginCodeStore holds single-use studio login codes (MAX /login → web exchange).
type LoginCodeStore struct {
	mu    sync.Mutex
	codes map[string]loginEntry
}

func NewLoginCodeStore() *LoginCodeStore {
	return &LoginCodeStore{codes: make(map[string]loginEntry)}
}

// Issue creates a one-time code (default TTL 15m).
func (s *LoginCodeStore) Issue(userID string, ttl time.Duration) (string, error) {
	if ttl <= 0 {
		ttl = 15 * time.Minute
	}
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	code := hex.EncodeToString(b)
	s.mu.Lock()
	s.gcLocked()
	s.codes[code] = loginEntry{UserID: userID, Exp: time.Now().Add(ttl)}
	s.mu.Unlock()
	return code, nil
}

// Consume validates and deletes the code. Returns userID.
func (s *LoginCodeStore) Consume(code string) (userID string, ok bool) {
	if code == "" {
		return "", false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.gcLocked()
	e, found := s.codes[code]
	if !found || time.Now().After(e.Exp) {
		delete(s.codes, code)
		return "", false
	}
	delete(s.codes, code)
	return e.UserID, true
}

func (s *LoginCodeStore) gcLocked() {
	now := time.Now()
	for k, e := range s.codes {
		if now.After(e.Exp) {
			delete(s.codes, k)
		}
	}
}
