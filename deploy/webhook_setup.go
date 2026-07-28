//go:build ignore

// Usage: go run deploy/webhook_setup.go -url https://uvo.example.com/api/max/webhook
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()
	webhookURL := flag.String("url", "", "HTTPS webhook URL")
	flag.Parse()
	if *webhookURL == "" {
		fmt.Println("usage: go run deploy/webhook_setup.go -url https://your.domain/api/max/webhook")
		os.Exit(1)
	}
	token := os.Getenv("MAX_BOT_TOKEN")
	base := os.Getenv("MAX_API_BASE")
	if base == "" {
		base = "https://platform-api2.max.ru"
	}
	if token == "" {
		fmt.Println("MAX_BOT_TOKEN required")
		os.Exit(1)
	}

	// POST /subscriptions — per MAX docs
	payload := map[string]interface{}{
		"url":          *webhookURL,
		"update_types": []string{"message_created", "bot_started", "message_callback"},
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", base+"/subscriptions", bytes.NewReader(body))
	if err != nil {
		panic(err)
	}
	req.Header.Set("Authorization", token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	fmt.Printf("status %d\n%s\n", resp.StatusCode, string(b))
	if resp.StatusCode >= 300 {
		os.Exit(1)
	}
	fmt.Println("OK: webhook subscribed. Disable long-poll: BOT_MODE=webhook")
}
