package web

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"uvo/internal/middleware"
	"uvo/internal/models"
	"uvo/internal/services"
)

func (d *Deps) getCredits(c *gin.Context) {
	uid := middleware.UserID(c)
	demo := os.Getenv("DEMO_TOPUP") == "true"
	pay := "coming_soon"
	note := "Оплата картой скоро. Задайте YOOKASSA_SHOP_ID и YOOKASSA_SECRET_KEY."
	if d.Yoo != nil && d.Yoo.Enabled() {
		pay = "yookassa"
		note = "Оплата картой через ЮKassa. Выберите пакет — откроется страница оплаты."
	} else if demo {
		pay = "demo"
		note = "Демо-пополнение (DEMO_TOPUP=true), без реальных денег."
	}
	hint := "1 кредит = 1 песня (генерация). Кавер/караоке/клон — 2."
	if services.CreditsUnlimited() {
		hint = "Тест: CREDITS_UNLIMITED — списание и лимиты отключены."
		note = "Режим тестирования: кредиты не списываются."
	}
	if uid == "" {
		note = "Нужен вход: кнопка «Запуск» в MAX или «Демо-вход»."
		if services.DemoGuestAuthEnabled() {
			hint = "Демо доступно: нажмите «Демо-вход» — у каждого своя сессия."
		}
	}
	c.JSON(200, gin.H{
		"balance":           d.Credits.Balance(uid),
		"authenticated":     uid != "",
		"packs":             services.PacksPublic(),
		"free_credits":      services.FreeCredits,
		"credit_hint":       hint,
		"dual_policy":       services.DualPolicyLabel(),
		"demo_topup":        demo,
		"demo_guest":        services.DemoGuestAuthEnabled(),
		"payment":           pay,
		"note":              note,
		"credits_unlimited": services.CreditsUnlimited(),
	})
}

func (d *Deps) topupCredits(c *gin.Context) {
	if os.Getenv("DEMO_TOPUP") != "true" {
		middleware.AbortJSON(c, 403, "forbidden", "demo topup disabled (set DEMO_TOPUP=true)")
		return
	}
	uid := middleware.UserID(c)
	var req struct {
		PackID  string `json:"pack_id"`
		Credits int    `json:"credits"`
	}
	_ = c.ShouldBindJSON(&req)
	n := req.Credits
	if credits, _, _, ok := services.PackByID(req.PackID); ok {
		n = credits
	}
	if n <= 0 {
		n = 10
	}
	if n > 1000 {
		n = 1000
	}
	if err := d.Credits.Add(uid, n); err != nil {
		middleware.AbortJSON(c, 500, "internal_error", "не удалось начислить кредиты")
		return
	}
	c.JSON(200, gin.H{"balance": d.Credits.Balance(uid), "added": n, "note": "demo topup without payment"})
}

func (d *Deps) checkoutCredits(c *gin.Context) {
	uid := middleware.UserID(c)
	var req struct {
		PackID string `json:"pack_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.AbortJSON(c, 400, "validation_error", "pack_id обязателен")
		return
	}
	credits, price, name, ok := services.PackByID(req.PackID)
	if !ok {
		middleware.AbortJSON(c, 400, "validation_error", "неизвестный pack_id")
		return
	}
	if d.Yoo == nil || !d.Yoo.Enabled() {
		middleware.AbortJSON(c, 503, "payment_unavailable", "ЮKassa не настроена (YOOKASSA_SHOP_ID / YOOKASSA_SECRET_KEY)")
		return
	}
	orderID := uuid.New().String()
	order := &models.PaymentOrder{
		ID:        orderID,
		UserID:    uid,
		PackID:    req.PackID,
		Credits:   credits,
		AmountRub: price,
		Status:    "pending",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := d.DB.Create(order).Error; err != nil {
		middleware.AbortJSON(c, 500, "internal_error", "не удалось создать заказ")
		return
	}
	ret := ""
	if d.Cfg != nil && d.Cfg.WebPublicURL != "" {
		ret = strings.TrimRight(d.Cfg.WebPublicURL, "/") + "/?paid=1#pricing"
	}
	if ret == "" {
		middleware.AbortJSON(c, 503, "payment_unavailable", "WEB_PUBLIC_URL обязателен для checkout")
		return
	}
	desc := fmt.Sprintf("UVO %s (%d кредитов)", name, credits)
	pay, err := d.Yoo.CreatePayment(price, desc, ret, orderID, uid, req.PackID, credits)
	if err != nil {
		_ = d.DB.Model(order).Updates(map[string]interface{}{"status": "canceled", "updated_at": time.Now()}).Error
		middleware.AbortJSON(c, 502, "payment_failed", "не удалось создать платёж")
		return
	}
	_ = d.DB.Model(order).Updates(map[string]interface{}{
		"provider_payment_id": pay.ID,
		"updated_at":          time.Now(),
	}).Error
	c.JSON(200, gin.H{
		"order_id":         orderID,
		"payment_id":       pay.ID,
		"confirmation_url": pay.ConfirmationURL,
		"amount_rub":       price,
		"credits":          credits,
	})
}

func (d *Deps) yooWebhook(c *gin.Context) {
	if !yooWebhookIPAllowed(c.ClientIP()) {
		logrus.WithField("ip", c.ClientIP()).Warn("yookassa webhook IP rejected")
		c.Status(403)
		return
	}
	raw, err := io.ReadAll(io.LimitReader(c.Request.Body, 1<<20))
	if err != nil {
		c.Status(400)
		return
	}
	var evt struct {
		Event  string `json:"event"`
		Object struct {
			ID       string                 `json:"id"`
			Status   string                 `json:"status"`
			Metadata map[string]interface{} `json:"metadata"`
		} `json:"object"`
	}
	if err := json.Unmarshal(raw, &evt); err != nil {
		c.Status(400)
		return
	}
	if evt.Event != "payment.succeeded" && evt.Object.Status != "succeeded" {
		c.JSON(200, gin.H{"ok": true})
		return
	}
	if d.Yoo == nil || !d.Yoo.Enabled() {
		logrus.Warn("yookassa webhook rejected: client not configured")
		c.Status(503)
		return
	}
	paymentID := strings.TrimSpace(evt.Object.ID)
	if paymentID == "" {
		c.Status(400)
		return
	}

	// Authoritative check — never trust webhook body alone.
	info, err := d.Yoo.GetPayment(paymentID)
	if err != nil {
		logrus.WithError(err).Warn("yookassa GetPayment failed")
		c.Status(502)
		return
	}
	if info.Status != "succeeded" && !info.Paid {
		c.JSON(200, gin.H{"ok": true, "ignored": "not_succeeded"})
		return
	}

	orderID := info.Metadata["order_id"]
	if orderID == "" {
		if v, ok := evt.Object.Metadata["order_id"].(string); ok {
			orderID = v
		}
	}
	if orderID == "" {
		c.JSON(200, gin.H{"ok": true})
		return
	}

	var order models.PaymentOrder
	if err := d.DB.First(&order, "id = ?", orderID).Error; err != nil {
		c.JSON(200, gin.H{"ok": true})
		return
	}
	if order.Status == "succeeded" {
		c.JSON(200, gin.H{"ok": true, "idempotent": true})
		return
	}
	if order.ProviderPaymentID != "" && order.ProviderPaymentID != paymentID {
		logrus.WithFields(logrus.Fields{"order": orderID, "want": order.ProviderPaymentID, "got": paymentID}).
			Warn("yookassa payment id mismatch")
		c.Status(403)
		return
	}
	if order.AmountRub > 0 && info.AmountRub > 0 && info.AmountRub != order.AmountRub {
		logrus.WithFields(logrus.Fields{"order": orderID, "want": order.AmountRub, "got": info.AmountRub}).
			Warn("yookassa amount mismatch")
		c.Status(403)
		return
	}
	if metaUID := info.Metadata["user_id"]; metaUID != "" && metaUID != order.UserID {
		logrus.Warn("yookassa user_id metadata mismatch")
		c.Status(403)
		return
	}

	credits := order.Credits
	if credits <= 0 {
		if v := info.Metadata["credits"]; v != "" {
			credits, _ = strconv.Atoi(v)
		}
	}
	if credits <= 0 {
		c.JSON(200, gin.H{"ok": true})
		return
	}

	settled, err := d.Credits.SettlePaymentCAS(orderID, paymentID, credits)
	if err != nil {
		logrus.WithError(err).Error("settle payment failed")
		c.Status(500)
		return
	}
	c.JSON(200, gin.H{"ok": true, "settled": settled})
}

func (d *Deps) listPresets(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"presets": services.ViralPresets})
}

// yooWebhookIPAllowed: if YOOKASSA_WEBHOOK_IPS is set (comma-separated IPs/CIDRs),
// only those clients may hit the webhook. Empty = allow all (GetPayment still required).
func yooWebhookIPAllowed(clientIP string) bool {
	raw := strings.TrimSpace(os.Getenv("YOOKASSA_WEBHOOK_IPS"))
	if raw == "" {
		return true
	}
	ip := net.ParseIP(strings.TrimSpace(clientIP))
	if ip == nil {
		return false
	}
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if strings.Contains(part, "/") {
			_, n, err := net.ParseCIDR(part)
			if err == nil && n.Contains(ip) {
				return true
			}
			continue
		}
		if p := net.ParseIP(part); p != nil && p.Equal(ip) {
			return true
		}
	}
	return false
}
