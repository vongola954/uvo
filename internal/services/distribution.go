package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"uvo/internal/models"
	"uvo/internal/repository"
)

const DistributionCost = 5

type DistributionService struct {
	db        *gorm.DB
	trackRepo *repository.TrackRepository
	webhook   string
}

func NewDistributionService(db *gorm.DB, tracks *repository.TrackRepository) *DistributionService {
	return &DistributionService{
		db:        db,
		trackRepo: tracks,
		webhook:   strings.TrimSpace(os.Getenv("DISTRIBUTION_WEBHOOK_URL")),
	}
}

type DistributeRequest struct {
	UserID    string
	TrackID   uint
	Title     string
	Artist    string
	Genre     string
	Platforms []string
	Notes     string
}

func (s *DistributionService) Submit(req *DistributeRequest) (*models.DistributionRelease, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("distribution unavailable")
	}
	track, err := s.trackRepo.GetByID(req.TrackID)
	if err != nil || track == nil || track.UserID != req.UserID {
		return nil, fmt.Errorf("track not found")
	}
	plats := normalizePlatforms(req.Platforms)
	if len(plats) == 0 {
		plats = []string{"spotify", "yandex", "apple", "vk"}
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = track.Title
	}
	artist := strings.TrimSpace(req.Artist)
	if artist == "" {
		artist = "UVO Artist"
	}
	rel := &models.DistributionRelease{
		ID:           uuid.New().String(),
		UserID:       req.UserID,
		TrackID:      track.ID,
		Title:        title,
		Artist:       artist,
		Genre:        firstNonEmpty(req.Genre, track.Genre),
		Platforms:    strings.Join(plats, ","),
		Status:       "queued",
		Notes:        strings.TrimSpace(req.Notes),
		CreditsSpent: DistributionCost,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	if err := s.db.Create(rel).Error; err != nil {
		return nil, err
	}
	if s.webhook != "" {
		if ext, err := s.pushWebhook(rel, track); err == nil && ext != "" {
			rel.ExternalID = ext
			rel.Status = "submitted"
			rel.UpdatedAt = time.Now()
			_ = s.db.Save(rel).Error
		} else if err != nil {
			rel.Notes = strings.TrimSpace(rel.Notes + " | webhook: " + err.Error())
			rel.Status = "queued"
			rel.UpdatedAt = time.Now()
			_ = s.db.Save(rel).Error
		}
	}
	return rel, nil
}

func (s *DistributionService) List(userID string) ([]models.DistributionRelease, error) {
	var list []models.DistributionRelease
	err := s.db.Where("user_id = ?", userID).Order("created_at DESC").Limit(50).Find(&list).Error
	return list, err
}

func (s *DistributionService) ApplyWebhookUpdate(externalID, status, note string) error {
	if externalID == "" {
		return fmt.Errorf("external_id required")
	}
	updates := map[string]interface{}{"updated_at": time.Now()}
	if status != "" {
		updates["status"] = status
	}
	if note != "" {
		updates["notes"] = note
	}
	res := s.db.Model(&models.DistributionRelease{}).Where("external_id = ?", externalID).Updates(updates)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("release not found")
	}
	return nil
}

func (s *DistributionService) pushWebhook(rel *models.DistributionRelease, track *models.Track) (string, error) {
	payload := map[string]interface{}{
		"id":         rel.ID,
		"track_id":   rel.TrackID,
		"title":      rel.Title,
		"artist":     rel.Artist,
		"genre":      rel.Genre,
		"platforms":  strings.Split(rel.Platforms, ","),
		"audio_path": track.FilePath,
		"user_id":    rel.UserID,
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", s.webhook, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if secret := strings.TrimSpace(os.Getenv("DISTRIBUTION_WEBHOOK_SECRET")); secret != "" {
		req.Header.Set("X-UVO-Distribution-Secret", secret)
	}
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("partner status %d", resp.StatusCode)
	}
	var out struct {
		ExternalID string `json:"external_id"`
		ID         string `json:"id"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&out)
	if out.ExternalID != "" {
		return out.ExternalID, nil
	}
	return out.ID, nil
}

func normalizePlatforms(in []string) []string {
	allow := map[string]bool{"spotify": true, "yandex": true, "apple": true, "vk": true, "youtube": true}
	var out []string
	seen := map[string]bool{}
	for _, p := range in {
		p = strings.ToLower(strings.TrimSpace(p))
		if p == "yandex_music" || p == "яндекс" {
			p = "yandex"
		}
		if !allow[p] || seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	return out
}

func firstNonEmpty(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return strings.TrimSpace(a)
	}
	return strings.TrimSpace(b)
}
