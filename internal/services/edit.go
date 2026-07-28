package services

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"uvo/internal/clients"
	"uvo/internal/models"
	"uvo/internal/repository"
)

type EditService struct {
	ace       *clients.AceDataClient
	trackRepo *repository.TrackRepository
	db        *gorm.DB
	mediaRoot string
}

func NewEditService(ace *clients.AceDataClient, trackRepo *repository.TrackRepository, db *gorm.DB, mediaRoot string) *EditService {
	if mediaRoot == "" {
		mediaRoot = "./data/media"
	}
	return &EditService{ace: ace, trackRepo: trackRepo, db: db, mediaRoot: mediaRoot}
}

type EditRequest struct {
	UserID       string
	TrackID      uint
	Style        string
	Lyrics       string
	Prompt       string
	Instrumental bool
}

func (s *EditService) Edit(req *EditRequest) (*models.Track, error) {
	track, err := s.trackRepo.GetByID(req.TrackID)
	if err != nil || track == nil {
		return nil, fmt.Errorf("track not found")
	}
	if track.UserID != req.UserID {
		return nil, fmt.Errorf("forbidden")
	}

	prompt := req.Prompt
	if prompt == "" {
		prompt = track.Prompt
	}
	style := req.Style
	if style == "" {
		style = track.Genre
	}
	lyrics := req.Lyrics
	if lyrics == "" {
		lyrics = track.Lyrics
	}

	var count int64
	s.db.Model(&models.TrackRevision{}).Where("track_id = ?", track.ID).Count(&count)
	changes, _ := json.Marshal(map[string]string{"old_style": track.Genre, "old_prompt": track.Prompt})
	_ = s.db.Create(&models.TrackRevision{
		TrackID: track.ID, Version: int(count) + 1, Changes: string(changes),
		FilePath: track.FilePath, Prompt: track.Prompt, Style: track.Genre,
	}).Error

	aceReq := &clients.GenerateRequest{
		Custom: lyrics != "", Prompt: prompt, Lyric: lyrics, Style: style,
		Title: track.Title, Instrumental: req.Instrumental, Model: "chirp-v5-5",
	}
	// Only AceData persona_ids work for re-generation with same voice
	if track.VoiceProfileID != "" {
		var vp models.VoiceProfile
		if s.db.Where("user_id = ? AND voice_id = ?", req.UserID, track.VoiceProfileID).First(&vp).Error == nil {
			if vp.Provider == "acedata" {
				aceReq.PersonaID = track.VoiceProfileID
			}
		}
	}
	resp, err := s.ace.Generate(aceReq)
	if err != nil {
		return nil, err
	}

	_ = os.MkdirAll(s.mediaRoot, 0755)
	filename := uuid.New().String() + ".mp3"
	filePath := filepath.Join(s.mediaRoot, filename)
	if err := SafeDownload(resp.AudioURL, filePath, 30<<20); err != nil {
		return nil, fmt.Errorf("download: %w", err)
	}

	oldPath := track.FilePath
	track.FilePath = filePath
	track.Genre = style
	track.Prompt = prompt
	track.Lyrics = lyrics
	if resp.Duration > 0 {
		track.Duration = int(resp.Duration)
	}
	if err := s.db.Save(track).Error; err != nil {
		return nil, err
	}
	if oldPath != "" && oldPath != filePath {
		_ = os.Remove(oldPath)
	}
	return track, nil
}

func (s *EditService) Revisions(trackID uint, userID string) ([]models.TrackRevision, error) {
	track, err := s.trackRepo.GetByID(trackID)
	if err != nil || track == nil || track.UserID != userID {
		return nil, fmt.Errorf("forbidden")
	}
	var list []models.TrackRevision
	err = s.db.Where("track_id = ?", trackID).Order("version desc").Find(&list).Error
	return list, err
}
