package services

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"gorm.io/gorm"
	"uvo/internal/models"
)

// RateLimiter uses DB-backed events (survives restart; single-node safe).
type RateLimiter struct {
	db     *gorm.DB
	limitH int
	limitD int
}

func NewRateLimiter(db *gorm.DB) *RateLimiter {
	h, _ := strconv.Atoi(os.Getenv("GEN_RATE_LIMIT_HOUR"))
	d, _ := strconv.Atoi(os.Getenv("GEN_RATE_LIMIT_DAY"))
	if h <= 0 {
		h = 10
	}
	if d <= 0 {
		d = 40
	}
	return &RateLimiter{db: db, limitH: h, limitD: d}
}

// Allow records a generate event; rolls back the event if limits exceeded (insert-then-check).
func (r *RateLimiter) Allow(userID string) error {
	if CreditsUnlimited() {
		return nil
	}
	now := time.Now()
	hourAgo := now.Add(-time.Hour)
	dayAgo := now.Add(-24 * time.Hour)

	ev := &models.RateEvent{UserID: userID, Kind: "generate", CreatedAt: now}
	if err := r.db.Create(ev).Error; err != nil {
		return err
	}

	var hourN, dayN int64
	_ = r.db.Model(&models.RateEvent{}).
		Where("user_id = ? AND kind = ? AND created_at > ?", userID, "generate", hourAgo).
		Count(&hourN).Error
	_ = r.db.Model(&models.RateEvent{}).
		Where("user_id = ? AND kind = ? AND created_at > ?", userID, "generate", dayAgo).
		Count(&dayN).Error

	if int(hourN) > r.limitH {
		_ = r.db.Delete(ev).Error
		return fmt.Errorf("rate limit: max %d generations per hour", r.limitH)
	}
	if int(dayN) > r.limitD {
		_ = r.db.Delete(ev).Error
		return fmt.Errorf("rate limit: max %d generations per day", r.limitD)
	}
	return nil
}

func ClampDuration(sec, max int) int {
	if max <= 0 {
		max = 480
	}
	if sec <= 0 {
		return 180
	}
	if sec > max {
		return max
	}
	return sec
}
