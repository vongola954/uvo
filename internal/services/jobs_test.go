package services

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"uvo/internal/models"
)

func jobsTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := "file:jobs_" + t.Name() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.JobRecord{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestCreateOrClaimIdempotent(t *testing.T) {
	s := NewJobStore(jobsTestDB(t))
	j1, created1 := s.CreateOrClaim("u1", "req-a")
	if !created1 || j1 == nil {
		t.Fatal("first create should win")
	}
	j2, created2 := s.CreateOrClaim("u1", "req-a")
	if created2 {
		t.Fatal("second create must not own the job")
	}
	if j2.ID != j1.ID {
		t.Fatalf("want same job %s got %s", j1.ID, j2.ID)
	}
}

func TestCreateOrClaimConcurrentOneWinner(t *testing.T) {
	s := NewJobStore(jobsTestDB(t))
	var winners int32
	var wg sync.WaitGroup
	ids := make(chan string, 40)
	for i := 0; i < 40; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			j, created := s.CreateOrClaim("u1", "same-req")
			if created {
				atomic.AddInt32(&winners, 1)
			}
			ids <- j.ID
		}()
	}
	wg.Wait()
	close(ids)
	if winners != 1 {
		t.Fatalf("want exactly 1 created winner, got %d", winners)
	}
	first := ""
	for id := range ids {
		if first == "" {
			first = id
		} else if id != first {
			t.Fatalf("mismatched job ids %s vs %s", first, id)
		}
	}
}

func TestClaimProcessingCAS(t *testing.T) {
	s := NewJobStore(jobsTestDB(t))
	j, _ := s.CreateOrClaim("u1", "cas")
	var claimed int32
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if s.ClaimProcessing(j.ID) {
				atomic.AddInt32(&claimed, 1)
			}
		}()
	}
	wg.Wait()
	if claimed != 1 {
		t.Fatalf("want 1 claim, got %d", claimed)
	}
	got, ok := s.Get(j.ID)
	if !ok || got.Status != JobProcessing {
		t.Fatalf("status=%v ok=%v", got, ok)
	}
}

func TestDeletePendingAfterSpendFail(t *testing.T) {
	s := NewJobStore(jobsTestDB(t))
	j, created := s.CreateOrClaim("u1", "del")
	if !created {
		t.Fatal("expected create")
	}
	s.Delete(j.ID)
	if _, ok := s.Get(j.ID); ok {
		t.Fatal("expected deleted")
	}
	// retry can create again
	j2, created2 := s.CreateOrClaim("u1", "del")
	if !created2 {
		t.Fatal("expected recreate after delete")
	}
	if j2.ID == j.ID {
		t.Fatal("expected new id")
	}
}
