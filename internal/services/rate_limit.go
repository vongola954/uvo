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

func (r *RateLimiter) Allow(userID string) error {
	now := time.Now()
	hourAgo := now.Add(-time.Hour)
	dayAgo := now.Add(-24 * time.Hour)

	var hourN, dayN int64
	_ = r.db.Model(&models.RateEvent{}).
		Where("user_id = ? AND kind = ? AND created_at > ?", userID, "generate", hourAgo).
		Count(&hourN).Error
	_ = r.db.Model(&models.RateEvent{}).
		Where("user_id = ? AND kind = ? AND created_at > ?", userID, "generate", dayAgo).
		Count(&dayN).Error

	if int(hourN) >= r.limitH {
		return fmt.Errorf("rate limit: max %d generations per hour", r.limitH)
	}
	if int(dayN) >= r.limitD {
		return fmt.Errorf("rate limit: max %d generations per day", r.limitD)
	}

	return r.db.Create(&models.RateEvent{UserID: userID, Kind: "generate", CreatedAt: now}).Error
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
