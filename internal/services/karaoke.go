package services

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"uvo/internal/clients"
	"uvo/internal/models"
	"uvo/internal/repository"
)

type KaraokeService struct {
	ace       *clients.AceDataClient
	trackRepo *repository.TrackRepository
	db        *gorm.DB
	mediaRoot string
	publicURL string
}

func NewKaraokeService(ace *clients.AceDataClient, tracks *repository.TrackRepository, db *gorm.DB, mediaRoot, publicURL string) *KaraokeService {
	if mediaRoot == "" {
		mediaRoot = "./data/media"
	}
	return &KaraokeService{ace: ace, trackRepo: tracks, db: db, mediaRoot: mediaRoot, publicURL: publicURL}
}

type KaraokeResult struct {
	TrackID          uint                  `json:"track_id"`
	Title            string                `json:"title"`
	InstrumentalURL  string                `json:"instrumental_url"`
	VocalsURL        string                `json:"vocals_url,omitempty"`
	OriginalURL      string                `json:"original_url"`
	VideoURL         string                `json:"video_url,omitempty"`
	Timing           []clients.TimingWord  `json:"timing"`
	Lyrics           string                `json:"lyrics"`
	Cost             int                   `json:"cost"`
}

func (s *KaraokeService) ensureAudioID(track *models.Track) (string, error) {
	if track.ProviderAudioID != "" {
		return track.ProviderAudioID, nil
	}
	// Re-upload local file so AceData can stem it
	data, err := os.ReadFile(track.FilePath)
	if err != nil {
		return "", fmt.Errorf("нет AceData audio_id и локальный файл недоступен — сгенерируйте трек заново")
	}
	filename, _, err := PublicUpload(s.mediaRoot, data, ".mp3")
	if err != nil {
		return "", err
	}
	pub, err := PublicURL(s.publicURL, filename)
	if err != nil {
		return "", err
	}
	up, err := s.ace.UploadReference(pub)
	if err != nil {
		return "", fmt.Errorf("upload for karaoke: %w", err)
	}
	track.ProviderAudioID = up.AudioID
	if track.Lyrics == "" && up.Lyric != "" {
		track.Lyrics = up.Lyric
	}
	_ = s.db.Save(track).Error
	return up.AudioID, nil
}

func (s *KaraokeService) Build(userID string, trackID uint) (*KaraokeResult, error) {
	track, err := s.trackRepo.GetByID(trackID)
	if err != nil || track == nil || track.UserID != userID {
		return nil, fmt.Errorf("track not found")
	}
	audioID, err := s.ensureAudioID(track)
	if err != nil {
		return nil, err
	}

	_ = os.MkdirAll(s.mediaRoot, 0755)
	stems, err := s.ace.Stems(audioID)
	if err != nil {
		return nil, fmt.Errorf("stems: %w", err)
	}

	result := &KaraokeResult{
		TrackID:     track.ID,
		Title:       track.Title,
		OriginalURL: fmt.Sprintf("/tracks/%d/play", track.ID),
		Lyrics:      track.Lyrics,
		Cost:        2,
	}

	if stems.Instrumental != nil && stems.Instrumental.AudioURL != "" {
		name := uuid.New().String() + "_instr.mp3"
		path := filepath.Join(s.mediaRoot, name)
		if err := SafeDownload(stems.Instrumental.AudioURL, path, 40<<20); err != nil {
			return nil, err
		}
		track.InstrumentalPath = path
		result.InstrumentalURL = fmt.Sprintf("/tracks/%d/instrumental", track.ID)
	}
	if stems.Vocals != nil && stems.Vocals.AudioURL != "" {
		name := uuid.New().String() + "_vocals.mp3"
		path := filepath.Join(s.mediaRoot, name)
		if err := SafeDownload(stems.Vocals.AudioURL, path, 40<<20); err != nil {
			logrus.WithError(err).Warn("vocals download failed")
		} else {
			track.VocalsPath = path
			result.VocalsURL = fmt.Sprintf("/tracks/%d/vocals", track.ID)
		}
	}

	if vurl, err := s.ace.GetMP4(audioID); err == nil && vurl != "" {
		name := uuid.New().String() + ".mp4"
		path := filepath.Join(s.mediaRoot, name)
		if err := SafeDownload(vurl, path, 80<<20); err == nil {
			track.VideoPath = path
			result.VideoURL = fmt.Sprintf("/tracks/%d/video", track.ID)
		} else {
			result.VideoURL = vurl
			track.VideoPath = vurl
		}
	} else if track.VideoPath != "" {
		if strings.HasPrefix(track.VideoPath, "http") {
			result.VideoURL = track.VideoPath
		} else {
			result.VideoURL = fmt.Sprintf("/tracks/%d/video", track.ID)
		}
	}

	timing, err := s.ace.GetTiming(audioID)
	if err != nil {
		logrus.WithError(err).Warn("timing failed, karaoke without sync")
	} else if timing != nil {
		result.Timing = timing.Words
	}

	meta, _ := json.Marshal(map[string]interface{}{
		"audio_id": audioID,
		"timing":   result.Timing,
	})
	_ = s.db.Create(&models.MediaAsset{
		UserID: userID, TrackID: track.ID, Kind: "karaoke",
		FilePath: track.InstrumentalPath, MetaJSON: string(meta),
	}).Error
	_ = s.db.Save(track).Error

	if result.InstrumentalURL == "" {
		return nil, fmt.Errorf("не удалось получить инструментал для караоке")
	}
	return result, nil
}
