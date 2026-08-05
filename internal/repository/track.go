package repository

import (
	"errors"
	"fmt"

	"uvo/internal/models"

	"gorm.io/gorm"
)

var ErrForbidden = errors.New("forbidden")

type TrackRepository struct {
	db *gorm.DB
}

func NewTrackRepository(db *gorm.DB) *TrackRepository {
	return &TrackRepository{db: db}
}

func (r *TrackRepository) Create(track *models.Track) error {
	return r.db.Create(track).Error
}

// Delete removes a track row (orphan cleanup after late/refunded jobs).
func (r *TrackRepository) Delete(id uint) error {
	return r.db.Delete(&models.Track{}, id).Error
}

func (r *TrackRepository) GetByID(id uint) (*models.Track, error) {
	var track models.Track
	err := r.db.First(&track, id).Error
	if err != nil {
		return nil, err
	}
	return &track, nil
}

func (r *TrackRepository) GetByUserID(userID string) ([]models.Track, error) {
	var tracks []models.Track
	err := r.db.Where("user_id = ?", userID).Find(&tracks).Error
	return tracks, err
}

// SetPublic updates is_public for a track owned by userID.
func (r *TrackRepository) SetPublic(id uint, userID string, isPublic bool) (*models.Track, error) {
	track, err := r.GetByID(id)
	if err != nil || track == nil {
		return nil, fmt.Errorf("not found")
	}
	if track.UserID != userID {
		return nil, ErrForbidden
	}
	if err := r.db.Model(track).Updates(map[string]interface{}{"is_public": isPublic}).Error; err != nil {
		return nil, err
	}
	track.IsPublic = isPublic
	return track, nil
}

// ListPublic returns recently created public tracks (discover).
func (r *TrackRepository) ListPublic(limit int) ([]models.Track, error) {
	if limit <= 0 {
		limit = 30
	}
	var tracks []models.Track
	err := r.db.Where("is_public = ?", true).Order("created_at desc").Limit(limit).Find(&tracks).Error
	return tracks, err
}
