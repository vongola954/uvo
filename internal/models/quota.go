package models

import "time"

// RateEvent persists generation attempts for durable rate limits (Epoch 9).
type RateEvent struct {
	ID        uint      `gorm:"primaryKey"`
	UserID    string    `gorm:"index;not null"`
	Kind      string    `gorm:"index;not null"` // generate
	CreatedAt time.Time `gorm:"index"`
}

// VoiceCloneEvent persists voice clone attempts for daily quota.
type VoiceCloneEvent struct {
	ID        uint      `gorm:"primaryKey"`
	UserID    string    `gorm:"index;not null"`
	CreatedAt time.Time `gorm:"index"`
}
