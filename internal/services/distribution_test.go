package services

import (
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"uvo/internal/models"
	"uvo/internal/repository"
)

func TestDistributionSubmitQueued(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:dist_q?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.Track{}, &models.DistributionRelease{}); err != nil {
		t.Fatal(err)
	}
	tr := &models.Track{UserID: "u1", Title: "Song", Genre: "pop", CreatedAt: time.Now()}
	if err := db.Create(tr).Error; err != nil {
		t.Fatal(err)
	}
	svc := NewDistributionService(db, repository.NewTrackRepository(db))
	rel, err := svc.Submit(&DistributeRequest{UserID: "u1", TrackID: tr.ID, Artist: "Test"})
	if err != nil {
		t.Fatal(err)
	}
	if rel.Status != "queued" {
		t.Fatalf("status=%s", rel.Status)
	}
	if rel.Platforms == "" || rel.CreditsSpent != DistributionCost {
		t.Fatalf("platforms=%q spent=%d", rel.Platforms, rel.CreditsSpent)
	}
	list, err := svc.List("u1")
	if err != nil || len(list) != 1 {
		t.Fatalf("list=%v err=%v", list, err)
	}
}

func TestDistributionSubmitWrongUser(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:dist_w?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	_ = db.AutoMigrate(&models.Track{}, &models.DistributionRelease{})
	tr := &models.Track{UserID: "owner", Title: "X", CreatedAt: time.Now()}
	_ = db.Create(tr)
	svc := NewDistributionService(db, repository.NewTrackRepository(db))
	if _, err := svc.Submit(&DistributeRequest{UserID: "other", TrackID: tr.ID}); err == nil {
		t.Fatal("expected error")
	}
}
