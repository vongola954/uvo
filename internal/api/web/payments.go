package web

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

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
	c.JSON(200, gin.H{
		"balance":    d.Credits.Balance(uid),
		"packs":      services.CreditPacks,
		"demo_topup": demo,
		"payment":    pay,
		"note":       note,
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
	d.Credits.Add(uid, n)
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
	ret := "https://uvo-baskakovanton.amvera.io/?paid=1#pricing"
	if d.Cfg != nil && d.Cfg.WebPublicURL != "" {
		ret = d.Cfg.WebPublicURL + "/?paid=1#pricing"
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
	orderID, _ := evt.Object.Metadata["order_id"].(string)
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
		c.JSON(200, gin.H{"ok": true})
		return
	}
	credits := order.Credits
	if credits <= 0 {
		if v, ok := evt.Object.Metadata["credits"]; ok {
			switch t := v.(type) {
			case float64:
				credits = int(t)
			case string:
				credits, _ = strconv.Atoi(t)
			}
		}
	}
	if credits <= 0 {
		c.JSON(200, gin.H{"ok": true})
		return
	}
	res := d.DB.Model(&models.PaymentOrder{}).
		Where("id = ? AND status = ?", orderID, "pending").
		Updates(map[string]interface{}{
			"status":              "succeeded",
			"provider_payment_id": evt.Object.ID,
			"updated_at":          time.Now(),
		})
	if res.Error != nil || res.RowsAffected == 0 {
		c.JSON(200, gin.H{"ok": true})
		return
	}
	d.Credits.Add(order.UserID, credits)
	c.JSON(200, gin.H{"ok": true})
}

func (d *Deps) listPresets(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"presets": services.ViralPresets})
}
