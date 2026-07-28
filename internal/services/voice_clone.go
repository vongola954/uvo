package services

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"uvo/internal/clients"
	"uvo/internal/models"
	"uvo/internal/repository"
)

type VoiceCloneService struct {
	sfClient  *clients.SiliconFlowClient
	elClient  *clients.ElevenLabsClient
	voiceRepo *repository.VoiceProfileRepository
	userRepo  *repository.UserRepository
	db        *gorm.DB
	limitDay  int
}

func NewVoiceCloneService(
	sf *clients.SiliconFlowClient,
	el *clients.ElevenLabsClient,
	voiceRepo *repository.VoiceProfileRepository,
	userRepo *repository.UserRepository,
	db *gorm.DB,
) *VoiceCloneService {
	lim, _ := strconv.Atoi(os.Getenv("VOICE_CLONE_LIMIT_DAY"))
	if lim <= 0 {
		lim = 3
	}
	return &VoiceCloneService{
		sfClient:  sf,
		elClient:  el,
		voiceRepo: voiceRepo,
		userRepo:  userRepo,
		db:        db,
		limitDay:  lim,
	}
}

func (s *VoiceCloneService) providerOrder() []string {
	raw := strings.ToLower(strings.TrimSpace(os.Getenv("VOICE_CLONE_PROVIDER")))
	if raw == "" || raw == "auto" {
		return []string{"elevenlabs", "siliconflow"}
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	seen := map[string]bool{}
	for _, p := range parts {
		p = strings.TrimSpace(p)
		switch p {
		case "el":
			p = "elevenlabs"
		case "sf":
			p = "siliconflow"
		}
		if p != "elevenlabs" && p != "siliconflow" {
			continue
		}
		if seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	if len(out) == 0 {
		return []string{"siliconflow", "elevenlabs"}
	}
	return out
}

func (s *VoiceCloneService) cloneWith(provider string, audioData []byte, name string) (string, error) {
	switch provider {
	case "elevenlabs":
		if s.elClient == nil || !s.elClient.Enabled() {
			return "", fmt.Errorf("not configured")
		}
		return s.elClient.CloneVoice(audioData, name)
	case "siliconflow":
		if s.sfClient == nil {
			return "", fmt.Errorf("client nil")
		}
		return s.sfClient.CloneVoice(audioData, name)
	default:
		return "", fmt.Errorf("unknown provider")
	}
}

func normalizeName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

// findExisting — дедуп: тот же user + то же имя (без суффикса [provider])
func (s *VoiceCloneService) findExisting(userID, name string) (*models.VoiceProfile, error) {
	list, err := s.voiceRepo.GetByUserID(userID)
	if err != nil {
		return nil, err
	}
	want := normalizeName(name)
	for i := range list {
		n := list[i].Name
		// strip " [provider]" suffix
		if idx := strings.LastIndex(n, " ["); idx > 0 {
			n = n[:idx]
		}
		if normalizeName(n) == want && list[i].Active {
			return &list[i], nil
		}
	}
	return nil, nil
}

func (s *VoiceCloneService) checkQuota(userID string) error {
	dayAgo := time.Now().Add(-24 * time.Hour)
	var n int64
	_ = s.db.Model(&models.VoiceCloneEvent{}).
		Where("user_id = ? AND created_at > ?", userID, dayAgo).
		Count(&n).Error
	if int(n) >= s.limitDay {
		return fmt.Errorf("квота клонирования: максимум %d / сутки", s.limitDay)
	}
	return nil
}

func (s *VoiceCloneService) recordClone(userID string) {
	_ = s.db.Create(&models.VoiceCloneEvent{UserID: userID, CreatedAt: time.Now()}).Error
}

func (s *VoiceCloneService) Clone(userID, name string, audioData []byte) (*models.VoiceProfile, error) {
	if name == "" {
		name = "voice"
	}

	// 1) Дедупликация — не тратим API
	if existing, err := s.findExisting(userID, name); err != nil {
		return nil, err
	} else if existing != nil {
		logrus.WithFields(logrus.Fields{
			"user_id":  userID,
			"voice_id": existing.VoiceID,
			"name":     existing.Name,
		}).Info("Voice clone dedup: returning existing profile")
		return existing, nil
	}

	if len(audioData) < 1000 {
		return nil, fmt.Errorf("audio too short, need 10–30 seconds")
	}

	// 2) Квота
	if err := s.checkQuota(userID); err != nil {
		return nil, err
	}

	order := s.providerOrder()
	var voiceID, used string
	var errs []string

	for _, provider := range order {
		id, err := s.cloneWith(provider, audioData, name)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", provider, err))
			logrus.WithError(err).WithField("provider", provider).Warn("voice clone attempt failed")
			continue
		}
		if id == "" {
			errs = append(errs, provider+": empty voice_id")
			continue
		}
		voiceID = id
		used = provider
		break
	}

	if voiceID == "" {
		if len(errs) == 0 {
			return nil, fmt.Errorf("no voice provider configured")
		}
		return nil, fmt.Errorf("all providers failed: %s", strings.Join(errs, "; "))
	}

	s.recordClone(userID)

	profile := &models.VoiceProfile{
		UserID:  userID,
		Name:    fmt.Sprintf("%s [%s]", name, used),
		VoiceID: voiceID,
		Active:  true,
	}
	if err := s.voiceRepo.Create(profile); err != nil {
		return nil, err
	}
	logrus.WithFields(logrus.Fields{
		"user_id": userID, "voice_id": voiceID, "provider": used,
	}).Info("Voice cloned")
	return profile, nil
}

func (s *VoiceCloneService) List(userID string) ([]models.VoiceProfile, error) {
	return s.voiceRepo.GetByUserID(userID)
}

func (s *VoiceCloneService) TTS(voiceID, text string) ([]byte, error) {
	if s.elClient == nil || !s.elClient.Enabled() {
		return nil, fmt.Errorf("elevenlabs required for TTS")
	}
	return s.elClient.TextToSpeech(voiceID, text, "eleven_multilingual_v2")
}

// OwnsVoice reports whether voiceID belongs to userID (active profile).
func (s *VoiceCloneService) OwnsVoice(userID, voiceID string) bool {
	if userID == "" || voiceID == "" {
		return false
	}
	list, err := s.voiceRepo.GetByUserID(userID)
	if err != nil {
		return false
	}
	for _, v := range list {
		if v.VoiceID == voiceID {
			return true
		}
	}
	return false
}

func (s *VoiceCloneService) QuotaRemaining(userID string) int {
	dayAgo := time.Now().Add(-24 * time.Hour)
	var n int64
	_ = s.db.Model(&models.VoiceCloneEvent{}).
		Where("user_id = ? AND created_at > ?", userID, dayAgo).
		Count(&n).Error
	left := s.limitDay - int(n)
	if left < 0 {
		return 0
	}
	return left
}
