package repository

import (
	"uvo/internal/models"

	"gorm.io/gorm"
)

type VoiceProfileRepository struct {
	db *gorm.DB
}

func NewVoiceProfileRepository(db *gorm.DB) *VoiceProfileRepository {
	return &VoiceProfileRepository{db: db}
}

func (r *VoiceProfileRepository) Create(p *models.VoiceProfile) error {
	return r.db.Create(p).Error
}

func (r *VoiceProfileRepository) GetByUserID(userID string) ([]models.VoiceProfile, error) {
	var list []models.VoiceProfile
	err := r.db.Where("user_id = ? AND active = ?", userID, true).Find(&list).Error
	return list, err
}

func (r *VoiceProfileRepository) GetByID(id uint) (*models.VoiceProfile, error) {
	var p models.VoiceProfile
	err := r.db.First(&p, id).Error
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *VoiceProfileRepository) Delete(id uint, userID string) error {
	return r.db.Where("id = ? AND user_id = ?", id, userID).Delete(&models.VoiceProfile{}).Error
}
