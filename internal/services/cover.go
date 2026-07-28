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

type CoverService struct {
	ace       *clients.AceDataClient
	trackRepo *repository.TrackRepository
	voice     *VoiceCloneService
	mediaRoot string
	publicURL string
}

func NewCoverService(
	ace *clients.AceDataClient,
	trackRepo *repository.TrackRepository,
	voice *VoiceCloneService,
	mediaRoot, publicURL string,
) *CoverService {
	if mediaRoot == "" {
		mediaRoot = "./data/media"
	}
	return &CoverService{
		ace: ace, trackRepo: trackRepo, voice: voice,
		mediaRoot: mediaRoot, publicURL: publicURL,
	}
}

type CoverFromUploadRequest struct {
	UserID    string
	AudioData []byte
	Filename  string
	VoiceID   string // AceData persona_id (required for own-voice cover)
	Prompt    string
	Style     string
	Lyrics    string
	Title     string
}

func (s *CoverService) CoverFromUpload(req *CoverFromUploadRequest) (*models.Track, error) {
	if len(req.AudioData) < 5000 {
		return nil, fmt.Errorf("файл слишком короткий — нужен полноценный трек")
	}
	if len(req.AudioData) > 30<<20 {
		return nil, fmt.Errorf("файл больше 30 MB")
	}
	persona := ""
	if req.VoiceID != "" {
		persona = s.voice.AcePersonaID(req.UserID, req.VoiceID)
		if persona == "" {
			return nil, fmt.Errorf("для кавера нужен голос AceData (клон через студию); ElevenLabs/SF не подходят для Suno")
		}
	}

	ext := UploadExtFromName(req.Filename)
	if ext != ".mp3" && ext != ".wav" {
		// AceData upload prefers mp3; still try
		ext = ".mp3"
	}
	filename, _, err := PublicUpload(s.mediaRoot, req.AudioData, ext)
	if err != nil {
		return nil, fmt.Errorf("save upload: %w", err)
	}
	pub, err := PublicURL(s.publicURL, filename)
	if err != nil {
		return nil, err
	}

	up, err := s.ace.UploadReference(pub)
	if err != nil {
		return nil, fmt.Errorf("acedata upload: %w", err)
	}

	style := req.Style
	if style == "" {
		style = up.Style
	}
	lyric := req.Lyrics
	if lyric == "" {
		lyric = up.Lyric
	}
	prompt := req.Prompt
	if prompt == "" {
		prompt = "cover version"
		if style != "" {
			prompt = "cover in style: " + truncate(style, 200)
		}
	}
	title := req.Title
	if title == "" {
		title = up.Title
	}
	if title == "" {
		title = "Cover"
	}

	resp, err := s.ace.UploadCover(&clients.CoverRequest{
		AudioID:   up.AudioID,
		PersonaID: persona,
		Prompt:    prompt,
		Style:     style,
		Lyric:     lyric,
		Title:     title,
		Model:     "chirp-v5-5",
	})
	if err != nil {
		return nil, fmt.Errorf("upload_cover: %w", err)
	}

	_ = os.MkdirAll(s.mediaRoot, 0755)
	outName := uuid.New().String() + ".mp3"
	outPath := filepath.Join(s.mediaRoot, outName)
	if err := SafeDownload(resp.AudioURL, outPath, 30<<20); err != nil {
		return nil, fmt.Errorf("download cover: %w", err)
	}

	dur := int(resp.Duration)
	if dur <= 0 {
		dur = int(up.Duration)
	}
	trackTitle := resp.Title
	if trackTitle == "" {
		trackTitle = title
	}

	track := &models.Track{
		UserID:         req.UserID,
		Title:          trackTitle,
		FilePath:       outPath,
		Duration:       dur,
		Genre:          style,
		Prompt:         prompt,
		Lyrics:         lyric,
		VoiceProfileID: req.VoiceID,
	}
	if err := s.trackRepo.Create(track); err != nil {
		return nil, err
	}
	logrus.WithFields(logrus.Fields{
		"track_id": track.ID, "user_id": req.UserID, "audio_id": up.AudioID, "persona": persona != "",
	}).Info("Cover created from upload")
	return track, nil
}

func GuessAudioContentType(name string) string {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".wav":
		return "audio/wav"
	case ".m4a":
		return "audio/mp4"
	case ".ogg":
		return "audio/ogg"
	default:
		return "audio/mpeg"
	}
}
