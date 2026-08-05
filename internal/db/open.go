package db

import (
	"fmt"
	"strings"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"uvo/internal/config"
	"uvo/internal/models"
)

func Open(cfg *config.Config) (*gorm.DB, error) {
	var dialector gorm.Dialector
	switch strings.ToLower(cfg.DBDriver) {
	case "postgres", "postgresql":
		if cfg.DatabaseURL == "" {
			return nil, fmt.Errorf("DATABASE_URL required when DB_DRIVER=postgres")
		}
		dialector = postgres.Open(cfg.DatabaseURL)
	default:
		dialector = sqlite.Open(cfg.DBPath)
	}

	gdb, err := gorm.Open(dialector, &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, err
	}
	if err := AutoMigrate(gdb); err != nil {
		return nil, err
	}
	return gdb, nil
}

func AutoMigrate(gdb *gorm.DB) error {
	if err := gdb.AutoMigrate(
		&models.User{}, &models.Track{}, &models.VoiceProfile{},
		&models.Playlist{}, &models.PlaylistTrack{}, &models.TrackRevision{},
		&models.SocialPost{}, &models.Like{}, &models.Comment{},
		&models.Subscription{}, &models.License{}, &models.Referral{},
		&models.CreditBalance{},
		&models.JobRecord{},
		&models.RateEvent{},
		&models.VoiceCloneEvent{},
		&models.MediaAsset{},
		&models.PaymentOrder{},
		&models.DistributionRelease{},
	); err != nil {
		return err
	}
	// Backfill idem_key for pre-2.2 rows (uniqueIndex cannot keep multiple empties).
	var jobs []models.JobRecord
	_ = gdb.Where("idem_key = '' OR idem_key IS NULL").Find(&jobs).Error
	for i := range jobs {
		key := jobs[i].ID
		if jobs[i].RequestID != "" {
			key = jobs[i].UserID + "|" + jobs[i].RequestID
		}
		_ = gdb.Model(&jobs[i]).Update("idem_key", key).Error
	}
	return nil
}
