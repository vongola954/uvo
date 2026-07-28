package repository

import (
	"uvo/internal/models"

	"gorm.io/gorm"
)

type TrackRepository struct {
	db *gorm.DB
}

func NewTrackRepository(db *gorm.DB) *TrackRepository {
	return &TrackRepository{db: db}
}

func (r *TrackRepository) Create(track *models.Track) error {
	return r.db.Create(track).Error
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