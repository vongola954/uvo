package services

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"uvo/internal/clients"
	"uvo/internal/models"
	"uvo/internal/repository"
)

type GenerationService struct {
	aceClient *clients.AceDataClient
	trackRepo *repository.TrackRepository
	userRepo  *repository.UserRepository
}

func NewGenerationService(ace *clients.AceDataClient, trackRepo *repository.TrackRepository, userRepo *repository.UserRepository) *GenerationService {
	return &GenerationService{
		aceClient: ace,
		trackRepo: trackRepo,
		userRepo:  userRepo,
	}
}

type GenerateRequest struct {
	UserID       string
	Prompt       string
	Lyrics       string
	Style        string
	Key          string
	BPM          int
	Duration     int
	VoiceID      string // stored on Track
	PersonaID    string // AceData persona for generation
	Instrumental bool
	Title        string
}

func (s *GenerationService) Generate(req *GenerateRequest) (*models.Track, error) {
	tracks, err := s.GenerateAll(req)
	if err != nil {
		return nil, err
	}
	return tracks[0], nil
}

// GenerateAll saves 1–2 tracks when DUAL_OUTPUT=true and provider returns variants.
func (s *GenerationService) GenerateAll(req *GenerateRequest) ([]*models.Track, error) {
	user, err := s.userRepo.GetByUserID(req.UserID)
	if err != nil || user == nil {
		user = &models.User{UserID: req.UserID}
		if err := s.userRepo.Create(user); err != nil {
			return nil, fmt.Errorf("failed to create user: %w", err)
		}
	}
	_ = user

	reqTitle := req.Title
	if reqTitle == "" {
		reqTitle = truncate(req.Prompt, 60)
	}
	aceReq := &clients.GenerateRequest{
		Custom: req.Lyrics != "", Prompt: req.Prompt, Lyric: req.Lyrics, Style: req.Style,
		Title: reqTitle, Instrumental: req.Instrumental, Model: "chirp-v5-5", PersonaID: req.PersonaID,
	}
	clips, err := s.aceClient.GenerateAll(aceReq)
	if err != nil {
		return nil, fmt.Errorf("ace data generation failed: %w", err)
	}
	if err := os.MkdirAll(s.getMediaRoot(), 0755); err != nil {
		return nil, fmt.Errorf("failed to create media dir: %w", err)
	}

	var tracks []*models.Track
	for i, resp := range clips {
		filename := uuid.New().String() + ".mp3"
		filePath := filepath.Join(s.getMediaRoot(), filename)
		if err := SafeDownload(resp.AudioURL, filePath, 30<<20); err != nil {
			if i == 0 {
				return nil, fmt.Errorf("download failed: %w", err)
			}
			logrus.WithError(err).Warn("skip variant download")
			continue
		}
		title := resp.Title
		if title == "" {
			title = truncate(req.Prompt, 100)
		}
		if i > 0 {
			title = title + " · v2"
		}
		duration := req.Duration
		if resp.Duration > 0 {
			duration = int(resp.Duration)
		}
		track := &models.Track{
			UserID: req.UserID, Title: title, FilePath: filePath, Duration: duration,
			Genre: req.Style, Key: req.Key, BPM: req.BPM, Prompt: req.Prompt, Lyrics: req.Lyrics,
			Instrumental: req.Instrumental, VoiceProfileID: req.VoiceID, ProviderAudioID: resp.AudioID,
		}
		if resp.Lyric != "" && track.Lyrics == "" {
			track.Lyrics = resp.Lyric
		}
		if resp.VideoURL != "" {
			vidName := uuid.New().String() + ".mp4"
			vidPath := filepath.Join(s.getMediaRoot(), vidName)
			if err := SafeDownload(resp.VideoURL, vidPath, 80<<20); err == nil {
				track.VideoPath = vidPath
			} else {
				track.VideoPath = resp.VideoURL
			}
		}
		if err := s.trackRepo.Create(track); err != nil {
			if i == 0 {
				return nil, fmt.Errorf("failed to save track: %w", err)
			}
			continue
		}
		tracks = append(tracks, track)
	}
	if len(tracks) == 0 {
		return nil, fmt.Errorf("no tracks saved")
	}
	logrus.WithFields(logrus.Fields{"track_id": tracks[0].ID, "variants": len(tracks), "user_id": req.UserID}).
		Info("Track generated successfully")
	return tracks, nil
}

func (s *GenerationService) getMediaRoot() string {
	if root := os.Getenv("MEDIA_ROOT"); root != "" {
		return root
	}
	return "./data/media"
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}