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
	ID               uint      `json:"id" gorm:"primaryKey"`
	UserID           string    `json:"user_id" gorm:"not null;index"`
	Title            string    `json:"title" gorm:"not null"`
	FilePath         string    `json:"-" gorm:"not null"`
	Duration         int       `json:"duration"`
	Genre            string    `json:"genre,omitempty"`
	Key              string    `json:"key,omitempty"`
	BPM              int       `json:"bpm,omitempty"`
	VoiceProfileID   string    `json:"voice_profile_id,omitempty"`
	ProviderAudioID  string    `json:"provider_audio_id,omitempty"`
	VideoPath        string    `json:"video_path,omitempty"`
	InstrumentalPath string    `json:"instrumental_path,omitempty"`
	VocalsPath       string    `json:"vocals_path,omitempty"`
	IsPublic         bool      `json:"is_public" gorm:"default:false"`
	PlayCount        int       `json:"play_count" gorm:"default:0"`
	Prompt           string    `json:"prompt,omitempty"`
	Lyrics           string    `json:"lyrics,omitempty"`
	Instrumental     bool      `json:"instrumental" gorm:"default:false"`
	CreatedAt        time.Time `json:"created_at"`
}

// MediaAsset stores generated karaoke/portrait/video/upscale artifacts.
type MediaAsset struct {
	ID        uint      `gorm:"primaryKey"`
	UserID    string    `gorm:"not null;index"`
	TrackID   uint      `gorm:"index"` // 0 = not tied to a track
	Kind      string    `gorm:"not null;index"` // karaoke | portrait | video | upscale | animate
	FilePath  string
	MetaJSON  string // timing words, provider ids, etc.
	CreatedAt time.Time
}

// DistributionRelease is a Spotify/Yandex/Apple/VK release request (partner or manual).
type DistributionRelease struct {
	ID           string    `gorm:"primaryKey" json:"id"`
	UserID       string    `gorm:"index;not null" json:"user_id"`
	TrackID      uint      `gorm:"index;not null" json:"track_id"`
	Title        string    `json:"title"`
	Artist       string    `json:"artist"`
	Genre        string    `json:"genre,omitempty"`
	Platforms    string    `json:"platforms"` // comma: spotify,yandex,apple,vk
	Status       string    `gorm:"index" json:"status"` // draft|queued|submitted|live|rejected
	ExternalID   string    `json:"external_id,omitempty"`
	CoverPath    string    `json:"cover_path,omitempty"`
	Notes        string    `json:"notes,omitempty"`
	CreditsSpent int       `json:"credits_spent,omitempty"`
	CreatedAt    time.Time `gorm:"index" json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
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
	ID          uint      `json:"id" gorm:"primaryKey"`
	UserID      string    `json:"user_id" gorm:"not null;index"`
	Name        string    `json:"name" gorm:"not null"`
	Description string    `json:"description,omitempty"`
	IsPublic    bool      `json:"is_public" gorm:"default:false"`
	Likes       int       `json:"likes" gorm:"default:0"`
	CreatedAt   time.Time `json:"created_at"`
}

type PlaylistTrack struct {
	ID         uint `json:"id" gorm:"primaryKey"`
	PlaylistID uint `json:"playlist_id" gorm:"not null;index"`
	TrackID    uint `json:"track_id" gorm:"not null;index"`
	Position   int  `json:"position"`
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
