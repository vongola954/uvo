package bot

import (
	"testing"

	"uvo/internal/clients"
)

func TestResolveChatUserBotStartedPayload(t *testing.T) {
	u := clients.MAXUpdate{
		UpdateType: "bot_started",
		ChatID:     453319035,
		UserID:     218786173,
		User:       &clients.MAXUser{UserID: 218786173, Name: "Test"},
	}
	chatID, userID := resolveChatUser(u)
	if chatID != 453319035 || userID != 218786173 {
		t.Fatalf("chat=%d user=%d", chatID, userID)
	}
}

func TestResolveChatUserFallback(t *testing.T) {
	u := clients.MAXUpdate{UserID: 42}
	chatID, userID := resolveChatUser(u)
	if chatID != 42 || userID != 42 {
		t.Fatalf("chat=%d user=%d", chatID, userID)
	}
}
