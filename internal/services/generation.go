package services

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"uvo/internal/clients"
	"uvo/internal/models"
	"uvo/internal/repository"
)

type musicGenerator interface {
	GenerateAll(req *clients.GenerateRequest) ([]*clients.GenerateResponse, error)
}

type GenerationService struct {
	aceClient     musicGenerator
	aceMusic      musicGenerator
	musicProvider string // auto | acedata | acemusic
	trackRepo     *repository.TrackRepository
	userRepo      *repository.UserRepository
}

func NewGenerationService(
	ace *clients.AceDataClient,
	aceMusic *clients.AceMusicClient,
	musicProvider string,
	trackRepo *repository.TrackRepository,
	userRepo *repository.UserRepository,
) *GenerationService {
	mp := strings.ToLower(strings.TrimSpace(musicProvider))
	if mp == "" {
		mp = "auto"
	}
	var aceGen musicGenerator
	if ace != nil {
		aceGen = ace
	}
	var musicGen musicGenerator
	if aceMusic != nil && aceMusic.Enabled() {
		musicGen = aceMusic
	}
	return &GenerationService{
		aceClient:     aceGen,
		aceMusic:      musicGen,
		musicProvider: mp,
		trackRepo:     trackRepo,
		userRepo:      userRepo,
	}
}

type GenerateRequest struct {
	UserID       string
	Prompt       string
	Lyrics       string
	Style        string
	Key          string
	BPM          int
	Duration     int
	VoiceID      string // stored on Track
	PersonaID    string // AceData persona for generation
	Instrumental bool
	Title        string
	Provider     string // optional override: auto|acedata|acemusic
}

func (s *GenerationService) ProviderLabel() string {
	hasAce := s.aceClient != nil
	hasMusic := s.aceMusic != nil
	switch {
	case hasAce && hasMusic:
		return s.musicProvider + "+acedata+acemusic"
	case hasMusic:
		return "acemusic"
	case hasAce:
		return "acedata"
	default:
		return "none"
	}
}

func (s *GenerationService) Generate(req *GenerateRequest) (*models.Track, error) {
	tracks, err := s.GenerateAll(req)
	if err != nil {
		return nil, err
	}
	return tracks[0], nil
}

// GenerateAll saves 1–2 tracks when DUAL_OUTPUT=true and provider returns variants.
func (s *GenerationService) GenerateAll(req *GenerateRequest) ([]*models.Track, error) {
	user, err := s.userRepo.GetByUserID(req.UserID)
	if err != nil || user == nil {
		user = &models.User{UserID: req.UserID}
		if err := s.userRepo.Create(user); err != nil {
			return nil, fmt.Errorf("failed to create user: %w", err)
		}
	}
	_ = user

	reqTitle := req.Title
	if reqTitle == "" {
		reqTitle = truncate(req.Prompt, 60)
	}
	aceReq := &clients.GenerateRequest{
		Custom: req.Lyrics != "", Prompt: req.Prompt, Lyric: req.Lyrics, Style: req.Style,
		Title: reqTitle, Instrumental: req.Instrumental, Model: "chirp-v5-5", PersonaID: req.PersonaID,
		Duration: req.Duration,
	}

	clips, providerUsed, err := s.generateClips(aceReq, req.Provider)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(s.getMediaRoot(), 0755); err != nil {
		return nil, fmt.Errorf("failed to create media dir: %w", err)
	}

	var tracks []*models.Track
	for i, resp := range clips {
		filename := uuid.New().String() + ".mp3"
		filePath := filepath.Join(s.getMediaRoot(), filename)
		if err := SafeDownload(resp.AudioURL, filePath, 30<<20); err != nil {
			if i == 0 {
				return nil, fmt.Errorf("download failed: %w", err)
			}
			logrus.WithError(err).Warn("skip variant download")
			continue
		}
		title := resp.Title
		if title == "" {
			title = truncate(req.Prompt, 100)
		}
		if i > 0 {
			title = title + " · v2"
		}
		duration := req.Duration
		if resp.Duration > 0 {
			duration = int(resp.Duration)
		}
		track := &models.Track{
			UserID: req.UserID, Title: title, FilePath: filePath, Duration: duration,
			Genre: req.Style, Key: req.Key, BPM: req.BPM, Prompt: req.Prompt, Lyrics: req.Lyrics,
			Instrumental: req.Instrumental, VoiceProfileID: req.VoiceID, ProviderAudioID: resp.AudioID,
		}
		if resp.Lyric != "" && track.Lyrics == "" {
			track.Lyrics = resp.Lyric
		}
		if resp.VideoURL != "" {
			vidName := uuid.New().String() + ".mp4"
			vidPath := filepath.Join(s.getMediaRoot(), vidName)
			if err := SafeDownload(resp.VideoURL, vidPath, 80<<20); err == nil {
				track.VideoPath = vidPath
			} else {
				track.VideoPath = resp.VideoURL
			}
		}
		if err := s.trackRepo.Create(track); err != nil {
			if i == 0 {
				return nil, fmt.Errorf("failed to save track: %w", err)
			}
			continue
		}
		tracks = append(tracks, track)
	}
	if len(tracks) == 0 {
		return nil, fmt.Errorf("no tracks saved")
	}
	logrus.WithFields(logrus.Fields{
		"track_id": tracks[0].ID, "variants": len(tracks), "user_id": req.UserID, "provider": providerUsed,
	}).Info("Track generated successfully")
	return tracks, nil
}

func (s *GenerationService) generateClips(req *clients.GenerateRequest, override string) ([]*clients.GenerateResponse, string, error) {
	order := s.providerOrder(override)
	if len(order) == 0 {
		return nil, "", fmt.Errorf("no music provider configured")
	}
	var lastErr error
	for i, name := range order {
		gen := s.generatorByName(name)
		if gen == nil {
			continue
		}
		// AceMusic ignores AceData persona; strip to avoid confusing provider.
		callReq := *req
		if name == "acemusic" {
			callReq.PersonaID = ""
			callReq.Model = ""
		}
		clips, err := gen.GenerateAll(&callReq)
		if err == nil && len(clips) > 0 {
			if i > 0 {
				logrus.WithField("provider", name).Warn("music provider fallback succeeded")
			}
			return clips, name, nil
		}
		if err != nil {
			lastErr = err
			logrus.WithError(err).WithField("provider", name).Warn("music provider failed")
			if !shouldFallbackMusic(err) || i == len(order)-1 {
				return nil, name, fmt.Errorf("%s generation failed: %w", name, err)
			}
			continue
		}
		lastErr = fmt.Errorf("%s returned no clips", name)
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no music provider available")
	}
	return nil, "", lastErr
}

func (s *GenerationService) providerOrder(override string) []string {
	raw := strings.ToLower(strings.TrimSpace(override))
	if raw == "" {
		raw = s.musicProvider
	}
	switch raw {
	case "acedata":
		return filterConfigured([]string{"acedata"}, s)
	case "acemusic":
		return filterConfigured([]string{"acemusic"}, s)
	default: // auto
		return filterConfigured([]string{"acedata", "acemusic"}, s)
	}
}

func filterConfigured(names []string, s *GenerationService) []string {
	out := make([]string, 0, len(names))
	for _, n := range names {
		if s.generatorByName(n) != nil {
			out = append(out, n)
		}
	}
	return out
}

func (s *GenerationService) generatorByName(name string) musicGenerator {
	switch name {
	case "acedata":
		return s.aceClient
	case "acemusic":
		return s.aceMusic
	default:
		return nil
	}
}

func shouldFallbackMusic(err error) bool {
	if err == nil {
		return false
	}
	if pe := clients.AsProviderError(err); pe != nil {
		switch pe.Code {
		case "provider_balance_empty", "provider_auth", "provider_rate_limit", "provider_timeout":
			return true
		}
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "used_up") ||
		strings.Contains(msg, "not sufficient") ||
		strings.Contains(msg, "invalid_token") ||
		strings.Contains(msg, "timeout")
}

func (s *GenerationService) getMediaRoot() string {
	if root := os.Getenv("MEDIA_ROOT"); root != "" {
		return root
	}
	return "./data/media"
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}
