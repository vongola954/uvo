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
	// Ensure user exists
	user, err := s.userRepo.GetByUserID(req.UserID)
	if err != nil || user == nil {
		user = &models.User{UserID: req.UserID}
		if err := s.userRepo.Create(user); err != nil {
			return nil, fmt.Errorf("failed to create user: %w", err)
		}
	}

	reqTitle := req.Title
	if reqTitle == "" {
		reqTitle = truncate(req.Prompt, 60)
	}

	aceReq := &clients.GenerateRequest{
		Custom:       req.Lyrics != "",
		Prompt:       req.Prompt,
		Lyric:        req.Lyrics,
		Style:        req.Style,
		Title:        reqTitle,
		Instrumental: req.Instrumental,
		Model:        "chirp-v5-5",
		PersonaID:    req.PersonaID,
	}

	resp, err := s.aceClient.Generate(aceReq)
	if err != nil {
		return nil, fmt.Errorf("ace data generation failed: %w", err)
	}

	if err := os.MkdirAll(s.getMediaRoot(), 0755); err != nil {
		return nil, fmt.Errorf("failed to create media dir: %w", err)
	}

	filename := uuid.New().String() + ".mp3"
	filePath := filepath.Join(s.getMediaRoot(), filename)

	if err := SafeDownload(resp.AudioURL, filePath, 30<<20); err != nil {
		return nil, fmt.Errorf("download failed: %w", err)
	}

	title := resp.Title
	if title == "" {
		title = truncate(req.Prompt, 100)
	}
	duration := req.Duration
	if resp.Duration > 0 {
		duration = int(resp.Duration)
	}

	track := &models.Track{
		UserID:       req.UserID,
		Title:        title,
		FilePath:     filePath,
		Duration:     duration,
		Genre:        req.Style,
		Key:          req.Key,
		BPM:          req.BPM,
		Prompt:       req.Prompt,
		Lyrics:       req.Lyrics,
		Instrumental: req.Instrumental,
		VoiceProfileID: req.VoiceID,
	}

	if err := s.trackRepo.Create(track); err != nil {
		return nil, fmt.Errorf("failed to save track: %w", err)
	}

	logrus.WithFields(logrus.Fields{
		"track_id": track.ID,
		"user_id":  req.UserID,
	}).Info("Track generated successfully")

	return track, nil
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