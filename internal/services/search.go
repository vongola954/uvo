package services

import (
	"uvo/internal/models"
	"gorm.io/gorm"
)

type SearchService struct {
	db *gorm.DB
}

func NewSearchService(db *gorm.DB) *SearchService {
	return &SearchService{db: db}
}

func (s *SearchService) Tracks(q, userID string, publicOnly bool) ([]models.Track, error) {
	var tracks []models.Track
	tx := s.db.Model(&models.Track{})
	if publicOnly {
		tx = tx.Where("is_public = ?", true)
	} else {
		tx = tx.Where("user_id = ? OR is_public = ?", userID, true)
	}
	if q != "" {
		like := "%" + q + "%"
		tx = tx.Where("title LIKE ? OR genre LIKE ? OR prompt LIKE ?", like, like, like)
	}
	err := tx.Order("created_at desc").Limit(50).Find(&tracks).Error
	return tracks, err
}
