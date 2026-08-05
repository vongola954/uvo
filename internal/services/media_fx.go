package services

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"uvo/internal/clients"
	"uvo/internal/models"
)

const (
	UpscaleCost = 1
	AnimateCost = 2
	VideoCost   = 3
)

type MediaFXService struct {
	db        *gorm.DB
	kling     *clients.KlingClient
	veo       *clients.VeoClient
	upscale   *clients.UpscaleClient
	mediaRoot string
	publicURL string
}

func NewMediaFXService(db *gorm.DB, kling *clients.KlingClient, veo *clients.VeoClient, up *clients.UpscaleClient, mediaRoot, publicURL string) *MediaFXService {
	return &MediaFXService{db: db, kling: kling, veo: veo, upscale: up, mediaRoot: mediaRoot, publicURL: publicURL}
}

type MediaFXResult struct {
	Kind     string `json:"kind"`
	URL      string `json:"url"`
	Provider string `json:"provider"`
	Cost     int    `json:"cost"`
	Note     string `json:"note,omitempty"`
	AssetID  uint   `json:"asset_id,omitempty"`
}

func (s *MediaFXService) EnabledVideo() bool {
	return s != nil && ((s.veo != nil && s.veo.Enabled()) || (s.kling != nil && s.kling.Enabled()))
}

func (s *MediaFXService) EnabledUpscale() bool {
	return s != nil && s.upscale != nil && s.upscale.Enabled()
}

func (s *MediaFXService) EnabledAnimate() bool {
	return s != nil && s.kling != nil && s.kling.Enabled()
}

func (s *MediaFXService) Upscale(userID string, image []byte, filename string) (*MediaFXResult, error) {
	if !s.EnabledUpscale() {
		return nil, fmt.Errorf("апскейл не настроен: задайте REPLICATE_API_TOKEN (или SUNO_API_KEY для AceData fal)")
	}
	pubURL, _, err := s.publishImage(image, filename)
	if err != nil {
		return nil, err
	}
	outURL, err := s.upscale.UpscaleURL(pubURL)
	if err != nil {
		return nil, err
	}
	local, providerPath := s.materialize(outURL, ".png")
	path := local
	if path == "" {
		path = providerPath
	}
	id, _ := s.saveAsset(userID, 0, "upscale", path, "upscale", filename)
	return &MediaFXResult{Kind: "upscale", URL: mediaPlayURL(path, outURL), Provider: "upscale", Cost: UpscaleCost, AssetID: id,
		Note: "AI upscale · храните файл — ссылка может истечь"}, nil
}

func (s *MediaFXService) Animate(userID string, image []byte, filename, prompt string) (*MediaFXResult, error) {
	if !s.EnabledAnimate() {
		return nil, fmt.Errorf("оживление фото требует SUNO_API_KEY (AceData Kling)")
	}
	pubURL, _, err := s.publishImage(image, filename)
	if err != nil {
		return nil, err
	}
	if prompt == "" {
		prompt = "Subtle cinematic motion, natural blinking and breathing, soft camera push-in, high detail"
	}
	vurl, err := s.kling.ImageToVideo(pubURL, prompt, 5)
	if err != nil {
		return nil, err
	}
	local, remote := s.materialize(vurl, ".mp4")
	path := local
	if path == "" {
		path = remote
	}
	id, _ := s.saveAsset(userID, 0, "animate", path, "kling", prompt)
	return &MediaFXResult{Kind: "animate", URL: mediaPlayURL(path, vurl), Provider: "kling", Cost: AnimateCost, AssetID: id}, nil
}

func (s *MediaFXService) GenerateVideo(userID, prompt string, image []byte, filename string) (*MediaFXResult, error) {
	if !s.EnabledVideo() {
		return nil, fmt.Errorf("видео: задайте VEO_API_KEY/GEMINI_API_KEY или SUNO_API_KEY (Kling)")
	}
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return nil, fmt.Errorf("нужен текстовый промпт")
	}
	var vurl, provider string
	var err error
	if len(image) > 0 && s.kling != nil && s.kling.Enabled() {
		pub, _, e := s.publishImage(image, filename)
		if e != nil {
			return nil, e
		}
		vurl, err = s.kling.ImageToVideo(pub, prompt, 10)
		provider = "kling"
	} else if s.veo != nil && s.veo.Enabled() {
		vurl, err = s.veo.TextToVideo(prompt)
		provider = "veo"
	} else if s.kling != nil && s.kling.Enabled() {
		vurl, err = s.kling.TextToVideo(prompt, 10)
		provider = "kling"
	} else {
		return nil, fmt.Errorf("нет доступного видео-провайдера")
	}
	if err != nil {
		// Veo fail → Kling text2video
		if provider == "veo" && s.kling != nil && s.kling.Enabled() {
			logrus.WithError(err).Warn("veo failed, falling back to kling")
			vurl, err = s.kling.TextToVideo(prompt, 10)
			provider = "kling"
		}
		if err != nil {
			return nil, err
		}
	}
	local, remote := s.materialize(vurl, ".mp4")
	path := local
	if path == "" {
		path = remote
	}
	id, _ := s.saveAsset(userID, 0, "video", path, provider, prompt)
	note := "AI video"
	if provider == "kling" {
		note = "Kling video (Veo-like). Для Google Veo задайте VEO_API_KEY/GEMINI_API_KEY."
	}
	return &MediaFXResult{Kind: "video", URL: mediaPlayURL(path, vurl), Provider: provider, Cost: VideoCost, AssetID: id, Note: note}, nil
}

func (s *MediaFXService) List(userID string, kind string) ([]models.MediaAsset, error) {
	q := s.db.Where("user_id = ?", userID)
	if kind != "" {
		q = q.Where("kind = ?", kind)
	} else {
		q = q.Where("kind IN ?", []string{"upscale", "animate", "video"})
	}
	var list []models.MediaAsset
	err := q.Order("created_at DESC").Limit(40).Find(&list).Error
	return list, err
}

func (s *MediaFXService) publishImage(data []byte, filename string) (publicURL, absPath string, err error) {
	ext := filepath.Ext(filename)
	if ext == "" {
		ext = ".jpg"
	}
	name, abs, err := PublicUpload(s.mediaRoot, data, ext)
	if err != nil {
		return "", "", err
	}
	url, err := PublicURL(s.publicURL, name)
	if err != nil {
		return "", "", err
	}
	return url, abs, nil
}

func (s *MediaFXService) materialize(remoteURL, ext string) (localPath, remote string) {
	remote = remoteURL
	if strings.TrimSpace(remoteURL) == "" {
		return "", ""
	}
	if err := os.MkdirAll(s.mediaRoot, 0755); err != nil {
		return "", remoteURL
	}
	out := filepath.Join(s.mediaRoot, uuid.New().String()+ext)
	if err := SafeDownload(remoteURL, out, 80<<20); err != nil {
		logrus.WithError(err).Warn("media_fx materialize failed")
		return "", remoteURL
	}
	return out, remoteURL
}

func (s *MediaFXService) saveAsset(userID string, trackID uint, kind, path, provider, meta string) (uint, error) {
	b, _ := json.Marshal(map[string]string{"provider": provider, "meta": meta, "at": time.Now().Format(time.RFC3339)})
	row := &models.MediaAsset{
		UserID: userID, TrackID: trackID, Kind: kind, FilePath: path, MetaJSON: string(b), CreatedAt: time.Now(),
	}
	if err := s.db.Create(row).Error; err != nil {
		return 0, err
	}
	return row.ID, nil
}

func mediaPlayURL(localOrRemote, fallback string) string {
	if strings.HasPrefix(localOrRemote, "http://") || strings.HasPrefix(localOrRemote, "https://") {
		return localOrRemote
	}
	if localOrRemote != "" {
		base := filepath.Base(localOrRemote)
		return "/media/assets/" + base
	}
	return fallback
}
