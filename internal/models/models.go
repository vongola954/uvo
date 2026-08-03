package models

import "time"

type User struct {
	ID        uint      `gorm:"primaryKey"`
	UserID    string    `gorm:"unique;not null;index"`
	Name      string
	Plan      string    `gorm:"default:'free'"` // free | pro | premier
	CreatedAt time.Time
}

type Track struct {
	ID             uint      `gorm:"primaryKey"`
	UserID         string    `gorm:"not null;index"`
	Title          string    `gorm:"not null"`
	FilePath       string    `gorm:"not null"`
	Duration       int
	Genre          string
	Key            string
	BPM            int
	VoiceProfileID string
	ProviderAudioID string // AceData/Suno audio id for stems/mp4/timing
	VideoPath       string // local or remote mp4 path/url
	InstrumentalPath string
	VocalsPath       string
	IsPublic       bool      `gorm:"default:false"`
	PlayCount      int       `gorm:"default:0"`
	Prompt         string
	Lyrics         string
	Instrumental   bool      `gorm:"default:false"`
	CreatedAt      time.Time
}

// MediaAsset stores generated karaoke/portrait artifacts.
type MediaAsset struct {
	ID        uint      `gorm:"primaryKey"`
	UserID    string    `gorm:"not null;index"`
	TrackID   uint      `gorm:"not null;index"`
	Kind      string    `gorm:"not null;index"` // karaoke | portrait
	FilePath  string
	MetaJSON  string // timing words, provider ids, etc.
	CreatedAt time.Time
}

type VoiceProfile struct {
	ID        uint      `gorm:"primaryKey"`
	UserID    string    `gorm:"not null;index"`
	Name      string    `gorm:"not null"`
	VoiceID   string    `gorm:"not null"` // AceData persona_id or EL/SF voice id
	Provider  string    `gorm:"default:'acedata'"` // acedata | elevenlabs | siliconflow
	Active    bool      `gorm:"default:true"`
	CreatedAt time.Time
}

type Playlist struct {
	ID        uint      `gorm:"primaryKey"`
	UserID    string    `gorm:"not null;index"`
	Name      string    `gorm:"not null"`
	Description string
	IsPublic  bool      `gorm:"default:false"`
	Likes     int       `gorm:"default:0"`
	CreatedAt time.Time
}

type PlaylistTrack struct {
	ID         uint `gorm:"primaryKey"`
	PlaylistID uint `gorm:"not null;index"`
	TrackID    uint `gorm:"not null;index"`
	Position   int
}

type TrackRevision struct {
	ID        uint      `gorm:"primaryKey"`
	TrackID   uint      `gorm:"not null;index"`
	Version   int
	Changes   string // JSON description of changes
	FilePath  string
	Prompt    string
	Style     string
	CreatedAt time.Time
}

type SocialPost struct {
	ID        uint      `gorm:"primaryKey"`
	UserID    string    `gorm:"not null;index"`
	TrackID   uint
	Caption   string
	Likes     int       `gorm:"default:0"`
	CreatedAt time.Time
}

type Like struct {
	ID     uint   `gorm:"primaryKey"`
	UserID string `gorm:"not null;uniqueIndex:idx_like_user_post"`
	PostID uint   `gorm:"not null;uniqueIndex:idx_like_user_post"`
}

type Comment struct {
	ID        uint      `gorm:"primaryKey"`
	UserID    string    `gorm:"not null"`
	PostID    uint      `gorm:"not null;index"`
	Text      string
	CreatedAt time.Time
}

type Subscription struct {
	ID        uint      `gorm:"primaryKey"`
	UserID    string    `gorm:"unique;not null"`
	Plan      string    // free | pro | premier
	ExpiresAt time.Time
	CreatedAt time.Time
}

type License struct {
	ID        uint      `gorm:"primaryKey"`
	TrackID   uint      `gorm:"not null"`
	BuyerID   string
	Type      string // exclusive | non-exclusive
	Price     float64
	CreatedAt time.Time
}

type Referral struct {
	ID         uint   `gorm:"primaryKey"`
	ReferrerID string `gorm:"not null;index"`
	ReferredID string `gorm:"not null"`
	Bonus      int
	CreatedAt  time.Time
}


type CreditBalance struct {
	ID        uint   `gorm:"primaryKey"`
	UserID    string `gorm:"unique;not null;index"`
	Balance   int    `gorm:"default:2"`
	UpdatedAt time.Time
}


type JobRecord struct {
	ID           string    `gorm:"primaryKey" json:"id"`
	UserID       string    `gorm:"index" json:"user_id"`
	Status       string    `gorm:"index" json:"status"`
	Error        string    `json:"error,omitempty"`
	TrackID      uint      `json:"track_id,omitempty"`
	AltTrackID   uint      `json:"alt_track_id,omitempty"`
	Title        string    `json:"title,omitempty"`
	PlayURL      string    `json:"play_url,omitempty"`
	AltPlayURL   string    `json:"alt_play_url,omitempty"`
	DownloadURL  string    `json:"download_url,omitempty"`
	Duration     int       `json:"duration,omitempty"`
	CreditsSpent int       `gorm:"default:0" json:"credits_spent,omitempty"`
	Refunded     bool      `gorm:"default:false;index" json:"refunded,omitempty"`
	RequestID    string    `gorm:"index" json:"request_id,omitempty"`
	IdemKey      string    `gorm:"uniqueIndex;not null" json:"-"`
	CreatedAt    time.Time `gorm:"index" json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// PaymentOrder tracks YooKassa (or demo) credit purchases.
type PaymentOrder struct {
	ID                string `gorm:"primaryKey"`
	UserID            string `gorm:"index;not null"`
	PackID            string `gorm:"index"`
	Credits           int
	AmountRub         int
	ProviderPaymentID string    `gorm:"index"`
	Status            string    `gorm:"index"` // pending | succeeded | canceled
	CreatedAt         time.Time `gorm:"index"`
	UpdatedAt         time.Time
}
