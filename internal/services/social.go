package services

import (
	"fmt"
	"strings"

	"gorm.io/gorm"

	"uvo/internal/models"
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
	// Publishing to the feed makes the track public.
	if !track.IsPublic {
		if err := s.db.Model(&track).Update("is_public", true).Error; err != nil {
			return nil, err
		}
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

// Feed returns posts whose tracks are public.
func (s *SocialService) Feed(limit int) ([]models.SocialPost, error) {
	if limit <= 0 {
		limit = 20
	}
	var posts []models.SocialPost
	err := s.db.Model(&models.SocialPost{}).
		Joins("JOIN tracks ON tracks.id = social_posts.track_id AND tracks.is_public = ?", true).
		Order("social_posts.created_at desc").
		Limit(limit).
		Find(&posts).Error
	return posts, err
}

func (s *SocialService) Like(userID string, postID uint) (*models.SocialPost, error) {
	var post models.SocialPost
	if err := s.db.First(&post, postID).Error; err != nil {
		return nil, fmt.Errorf("post not found")
	}
	like := models.Like{UserID: userID, PostID: postID}
	res := s.db.Where("user_id = ? AND post_id = ?", userID, postID).FirstOrCreate(&like)
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected > 0 {
		_ = s.db.Model(&post).Update("likes", gorm.Expr("likes + 1")).Error
		post.Likes++
	}
	return &post, nil
}

func (s *SocialService) Unlike(userID string, postID uint) (*models.SocialPost, error) {
	var post models.SocialPost
	if err := s.db.First(&post, postID).Error; err != nil {
		return nil, fmt.Errorf("post not found")
	}
	res := s.db.Where("user_id = ? AND post_id = ?", userID, postID).Delete(&models.Like{})
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected > 0 && post.Likes > 0 {
		_ = s.db.Model(&post).Update("likes", gorm.Expr("likes - 1")).Error
		post.Likes--
	}
	return &post, nil
}

func (s *SocialService) AddComment(userID string, postID uint, text string) (*models.Comment, error) {
	text = strings.TrimSpace(text)
	if text == "" || len(text) > 1000 {
		return nil, fmt.Errorf("invalid comment")
	}
	var post models.SocialPost
	if err := s.db.First(&post, postID).Error; err != nil {
		return nil, fmt.Errorf("post not found")
	}
	c := &models.Comment{UserID: userID, PostID: postID, Text: text}
	if err := s.db.Create(c).Error; err != nil {
		return nil, err
	}
	return c, nil
}

func (s *SocialService) Comments(postID uint, limit int) ([]models.Comment, error) {
	if limit <= 0 {
		limit = 50
	}
	var list []models.Comment
	err := s.db.Where("post_id = ?", postID).Order("created_at asc").Limit(limit).Find(&list).Error
	return list, err
}
