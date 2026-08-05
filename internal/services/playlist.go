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

func (s *PlaylistService) Create(userID, name, desc string, isPublic bool) (*models.Playlist, error) {
	p := &models.Playlist{UserID: userID, Name: name, Description: desc, IsPublic: isPublic}
	return p, s.db.Create(p).Error
}

func (s *PlaylistService) List(userID string) ([]models.Playlist, error) {
	var list []models.Playlist
	err := s.db.Where("user_id = ?", userID).Find(&list).Error
	return list, err
}

func (s *PlaylistService) SetPublic(userID string, playlistID uint, isPublic bool) (*models.Playlist, error) {
	var p models.Playlist
	if err := s.db.First(&p, playlistID).Error; err != nil {
		return nil, fmt.Errorf("not found")
	}
	if p.UserID != userID {
		return nil, fmt.Errorf("forbidden")
	}
	// map update: GORM must write false (zero value) for is_public
	if err := s.db.Model(&p).Updates(map[string]interface{}{"is_public": isPublic}).Error; err != nil {
		return nil, err
	}
	p.IsPublic = isPublic
	return &p, nil
}

func (s *PlaylistService) AddTrack(userID string, playlistID, trackID uint) error {
	if playlistID == 0 || trackID == 0 {
		return fmt.Errorf("нужны playlist_id и track_id")
	}
	var p models.Playlist
	if err := s.db.First(&p, playlistID).Error; err != nil {
		return fmt.Errorf("плейлист не найден")
	}
	if p.UserID != userID {
		return fmt.Errorf("это чужой плейлист — войдите тем же аккаунтом, которым создавали")
	}
	var t models.Track
	if err := s.db.First(&t, trackID).Error; err != nil {
		return fmt.Errorf("трек не найден")
	}
	if t.UserID != userID {
		return fmt.Errorf("трек другого аккаунта — создайте плейлист после входа через «Запуск»")
	}
	var n int64
	if err := s.db.Model(&models.PlaylistTrack{}).
		Where("playlist_id = ? AND track_id = ?", playlistID, trackID).
		Count(&n).Error; err != nil {
		return err
	}
	if n > 0 {
		return nil // already in playlist
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

// GetTracksForUser returns tracks if playlist is owned or public.
// Non-owners never receive private tracks through a public playlist.
func (s *PlaylistService) GetTracksForUser(userID string, playlistID uint) ([]models.Track, error) {
	var p models.Playlist
	if err := s.db.First(&p, playlistID).Error; err != nil {
		return nil, fmt.Errorf("not found")
	}
	if p.UserID != userID && !p.IsPublic {
		return nil, fmt.Errorf("forbidden")
	}
	tracks, err := s.GetTracks(playlistID)
	if err != nil {
		return nil, err
	}
	if p.UserID == userID {
		return tracks, nil
	}
	out := make([]models.Track, 0, len(tracks))
	for _, t := range tracks {
		if t.IsPublic {
			out = append(out, t)
		}
	}
	return out, nil
}
