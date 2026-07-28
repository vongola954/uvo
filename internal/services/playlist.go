package services

import (
	"fmt"

	"gorm.io/gorm"
	"uvo/internal/models"
)

type PlaylistService struct {
	db *gorm.DB
}

func NewPlaylistService(db *gorm.DB) *PlaylistService {
	return &PlaylistService{db: db}
}

func (s *PlaylistService) Create(userID, name, desc string) (*models.Playlist, error) {
	p := &models.Playlist{UserID: userID, Name: name, Description: desc}
	return p, s.db.Create(p).Error
}

func (s *PlaylistService) List(userID string) ([]models.Playlist, error) {
	var list []models.Playlist
	err := s.db.Where("user_id = ?", userID).Find(&list).Error
	return list, err
}

func (s *PlaylistService) AddTrack(userID string, playlistID, trackID uint) error {
	var p models.Playlist
	if err := s.db.First(&p, playlistID).Error; err != nil {
		return err
	}
	if p.UserID != userID {
		return fmt.Errorf("forbidden")
	}
	var t models.Track
	if err := s.db.First(&t, trackID).Error; err != nil {
		return err
	}
	if t.UserID != userID {
		return fmt.Errorf("forbidden track")
	}
	return s.db.Create(&models.PlaylistTrack{PlaylistID: playlistID, TrackID: trackID}).Error
}

func (s *PlaylistService) GetTracks(playlistID uint) ([]models.Track, error) {
	var pts []models.PlaylistTrack
	if err := s.db.Where("playlist_id = ?", playlistID).Find(&pts).Error; err != nil {
		return nil, err
	}
	var tracks []models.Track
	for _, pt := range pts {
		var t models.Track
		if s.db.First(&t, pt.TrackID).Error == nil {
			tracks = append(tracks, t)
		}
	}
	return tracks, nil
}

// GetTracksForUser returns tracks only if the playlist is owned by userID or is public.
func (s *PlaylistService) GetTracksForUser(userID string, playlistID uint) ([]models.Track, error) {
	var p models.Playlist
	if err := s.db.First(&p, playlistID).Error; err != nil {
		return nil, fmt.Errorf("not found")
	}
	if p.UserID != userID && !p.IsPublic {
		return nil, fmt.Errorf("forbidden")
	}
	return s.GetTracks(playlistID)
}
