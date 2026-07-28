package services

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"uvo/internal/clients"
	"uvo/internal/models"
	"uvo/internal/repository"
)

type PortraitService struct {
	hedra     *clients.HedraClient
	kling     *clients.KlingClient
	trackRepo *repository.TrackRepository
	db        *gorm.DB
	mediaRoot string
	publicURL string
}

func NewPortraitService(
	hedra *clients.HedraClient,
	kling *clients.KlingClient,
	tracks *repository.TrackRepository,
	db *gorm.DB,
	mediaRoot, publicURL string,
) *PortraitService {
	if mediaRoot == "" {
		mediaRoot = "./data/media"
	}
	return &PortraitService{
		hedra: hedra, kling: kling, trackRepo: tracks, db: db,
		mediaRoot: mediaRoot, publicURL: publicURL,
	}
}

type PortraitResult struct {
	TrackID   uint   `json:"track_id"`
	VideoURL  string `json:"video_url"`
	Provider  string `json:"provider"` // hedra | kling
	Note      string `json:"note,omitempty"`
	Cost      int    `json:"cost"`
}

func (s *PortraitService) Create(userID string, trackID uint, imageData []byte, imageName, prompt string) (*PortraitResult, error) {
	track, err := s.trackRepo.GetByID(trackID)
	if err != nil || track == nil || track.UserID != userID {
		return nil, fmt.Errorf("track not found")
	}
	if len(imageData) < 1000 {
		return nil, fmt.Errorf("нужно фото портрета (jpg/png)")
	}
	if len(imageData) > 10<<20 {
		return nil, fmt.Errorf("фото больше 10 MB")
	}
	_ = os.MkdirAll(s.mediaRoot, 0755)

	ext := strings.ToLower(filepath.Ext(imageName))
	if ext != ".png" && ext != ".jpg" && ext != ".jpeg" && ext != ".webp" {
		ext = ".jpg"
	}
	imgName := uuid.New().String() + ext
	imgPath := filepath.Join(s.mediaRoot, "uploads", imgName)
	_ = os.MkdirAll(filepath.Dir(imgPath), 0755)
	if err := os.WriteFile(imgPath, imageData, 0644); err != nil {
		return nil, err
	}

	// Prefer Hedra true lip-sync
	if s.hedra != nil && s.hedra.Enabled() {
		if track.FilePath == "" {
			return nil, fmt.Errorf("у трека нет аудиофайла")
		}
		url, err := s.hedra.GenerateAvatar(imgPath, track.FilePath, prompt)
		if err != nil {
			return nil, fmt.Errorf("hedra: %w", err)
		}
		out := filepath.Join(s.mediaRoot, uuid.New().String()+"_portrait.mp4")
		if err := SafeDownload(url, out, 100<<20); err != nil {
			_ = s.saveAsset(userID, track.ID, "portrait", url, "hedra", prompt)
			return &PortraitResult{TrackID: track.ID, VideoURL: url, Provider: "hedra", Cost: 3,
				Note: "lip-sync поющий портрет (Hedra)"}, nil
		}
		base := filepath.Base(out)
		_ = s.saveAsset(userID, track.ID, "portrait", out, "hedra", prompt)
		return &PortraitResult{
			TrackID: track.ID, VideoURL: "/media/assets/" + base,
			Provider: "hedra", Cost: 3,
			Note: "lip-sync поющий портрет (Hedra)",
		}, nil
	}

	// Fallback: Kling image2video (animated portrait, not true lip-sync)
	if s.kling == nil || !s.kling.Enabled() {
		return nil, fmt.Errorf("для поющего портрета нужен HEDRA_API_KEY (lip-sync) или рабочий AceData ключ для Kling-клипа")
	}
	pub, err := PublicURL(s.publicURL, imgName)
	if err != nil {
		return nil, err
	}
	if prompt == "" {
		prompt = "The person in the photo is singing passionately to the camera, natural mouth and emotion, music video portrait, cinematic"
	}
	vurl, err := s.kling.ImageToVideo(pub, prompt, 10)
	if err != nil {
		return nil, fmt.Errorf("kling: %w", err)
	}
	out := filepath.Join(s.mediaRoot, uuid.New().String()+"_portrait.mp4")
	playURL := vurl
	if err := SafeDownload(vurl, out, 100<<20); err == nil {
		playURL = "/media/assets/" + filepath.Base(out)
		_ = s.saveAsset(userID, track.ID, "portrait", out, "kling", prompt)
	} else {
		_ = s.saveAsset(userID, track.ID, "portrait", vurl, "kling", prompt)
		logrus.WithError(err).Warn("portrait video kept as remote URL")
	}
	return &PortraitResult{
		TrackID:  track.ID,
		VideoURL: playURL,
		Provider: "kling",
		Cost:     2,
		Note:     "портрет-клип Kling (не полный lip-sync). Для точного lip-sync задайте HEDRA_API_KEY.",
	}, nil
}

func (s *PortraitService) saveAsset(userID string, trackID uint, kind, path, provider, prompt string) error {
	meta, _ := json.Marshal(map[string]string{"provider": provider, "prompt": prompt})
	return s.db.Create(&models.MediaAsset{
		UserID: userID, TrackID: trackID, Kind: kind, FilePath: path, MetaJSON: string(meta),
	}).Error
}

func (s *PortraitService) Latest(userID string, trackID uint) (*models.MediaAsset, error) {
	var a models.MediaAsset
	err := s.db.Where("user_id = ? AND track_id = ? AND kind = ?", userID, trackID, "portrait").
		Order("id desc").First(&a).Error
	if err != nil {
		return nil, err
	}
	return &a, nil
}
