package clients

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
)

// YooKassaClient creates payments via YooKassa API v3.
type YooKassaClient struct {
	shopID string
	secret string
	http   *http.Client
}

func NewYooKassaClient() *YooKassaClient {
	shop := os.Getenv("YOOKASSA_SHOP_ID")
	secret := os.Getenv("YOOKASSA_SECRET_KEY")
	if shop == "" || secret == "" {
		return nil
	}
	return &YooKassaClient{
		shopID: shop,
		secret: secret,
		http:   &http.Client{Timeout: 30 * time.Second},
	}
}

func (y *YooKassaClient) Enabled() bool {
	return y != nil && y.shopID != "" && y.secret != ""
}

type YooPaymentResult struct {
	ID              string
	Status          string
	ConfirmationURL string
}

// YooPaymentInfo is a payment fetched from YooKassa (for webhook verification).
type YooPaymentInfo struct {
	ID       string
	Status   string
	Paid     bool
	AmountRub int
	Metadata map[string]string
}

// CreatePayment starts a redirect payment (bank card). amountRub in rubles.
func (y *YooKassaClient) CreatePayment(amountRub int, description, returnURL, orderID, userID, packID string, credits int) (*YooPaymentResult, error) {
	if !y.Enabled() {
		return nil, fmt.Errorf("yookassa not configured")
	}
	if amountRub < 1 {
		return nil, fmt.Errorf("invalid amount")
	}
	body := map[string]interface{}{
		"amount": map[string]interface{}{
			"value":    fmt.Sprintf("%d.00", amountRub),
			"currency": "RUB",
		},
		"capture": true,
		"confirmation": map[string]interface{}{
			"type":       "redirect",
			"return_url": returnURL,
		},
		"description": description,
		"metadata": map[string]interface{}{
			"order_id": orderID,
			"user_id":  userID,
			"pack_id":  packID,
			"credits":  credits,
		},
	}
	raw, _ := json.Marshal(body)
	req, err := http.NewRequest(http.MethodPost, "https://api.yookassa.ru/v3/payments", bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(y.shopID, y.secret)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotence-Key", uuid.New().String())

	resp, err := y.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("yookassa HTTP %d: %s", resp.StatusCode, truncateStr(string(data), 200))
	}
	var out struct {
		ID     string `json:"id"`
		Status string `json:"status"`
		Confirmation struct {
			ConfirmationURL string `json:"confirmation_url"`
		} `json:"confirmation"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	if out.Confirmation.ConfirmationURL == "" {
		return nil, fmt.Errorf("yookassa: empty confirmation_url")
	}
	return &YooPaymentResult{
		ID:              out.ID,
		Status:          out.Status,
		ConfirmationURL: out.Confirmation.ConfirmationURL,
	}, nil
}

// GetPayment fetches payment by id (authoritative source for webhooks).
func (y *YooKassaClient) GetPayment(paymentID string) (*YooPaymentInfo, error) {
	if !y.Enabled() {
		return nil, fmt.Errorf("yookassa not configured")
	}
	paymentID = strings.TrimSpace(paymentID)
	if paymentID == "" {
		return nil, fmt.Errorf("empty payment id")
	}
	req, err := http.NewRequest(http.MethodGet, "https://api.yookassa.ru/v3/payments/"+paymentID, nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(y.shopID, y.secret)
	resp, err := y.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("yookassa GET payment HTTP %d: %s", resp.StatusCode, truncateStr(string(data), 200))
	}
	var out struct {
		ID     string `json:"id"`
		Status string `json:"status"`
		Paid   bool   `json:"paid"`
		Amount struct {
			Value    string `json:"value"`
			Currency string `json:"currency"`
		} `json:"amount"`
		Metadata map[string]interface{} `json:"metadata"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	rub := 0
	if out.Amount.Value != "" {
		var f float64
		if _, err := fmt.Sscanf(out.Amount.Value, "%f", &f); err == nil {
			rub = int(f + 0.001)
		}
	}
	meta := map[string]string{}
	for k, v := range out.Metadata {
		switch t := v.(type) {
		case string:
			meta[k] = t
		case float64:
			meta[k] = fmt.Sprintf("%.0f", t)
		default:
			meta[k] = fmt.Sprint(v)
		}
	}
	return &YooPaymentInfo{
		ID: out.ID, Status: out.Status, Paid: out.Paid, AmountRub: rub, Metadata: meta,
	}, nil
}

func truncateStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
