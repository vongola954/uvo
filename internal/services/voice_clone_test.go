package services

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"uvo/internal/models"
)

func TestReserveQuotaConcurrent(t *testing.T) {
	dsn := "file:voice_quota_" + t.Name() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.VoiceCloneEvent{}); err != nil {
		t.Fatal(err)
	}
	s := &VoiceCloneService{db: db, limitDay: 3}
	var okN, failN int32
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := s.reserveQuota("u1"); err != nil {
				atomic.AddInt32(&failN, 1)
				return
			}
			atomic.AddInt32(&okN, 1)
		}()
	}
	wg.Wait()
	if okN != 3 {
		t.Fatalf("want 3 reserves, got ok=%d fail=%d", okN, failN)
	}
}
