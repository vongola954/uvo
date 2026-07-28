package services

import (
	"fmt"

	"uvo/internal/models"
	"gorm.io/gorm"
)

type SocialService struct {
	db *gorm.DB
}

func NewSocialService(db *gorm.DB) *SocialService {
	return &SocialService{db: db}
}

func (s *SocialService) CreatePost(userID string, trackID uint, caption string) (*models.SocialPost, error) {
	var track models.Track
	if err := s.db.First(&track, trackID).Error; err != nil {
		return nil, fmt.Errorf("track not found")
	}
	if track.UserID != userID {
		return nil, fmt.Errorf("forbidden: not your track")
	}
	post := &models.SocialPost{
		UserID:  userID,
		TrackID: trackID,
		Caption: caption,
	}
	if err := s.db.Create(post).Error; err != nil {
		return nil, err
	}
	return post, nil
}

func (s *SocialService) Feed(limit int) ([]models.SocialPost, error) {
	if limit <= 0 {
		limit = 20
	}
	var posts []models.SocialPost
	err := s.db.Order("created_at desc").Limit(limit).Find(&posts).Error
	return posts, err
}
