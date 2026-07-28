package services

import (
	"sync"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"uvo/internal/models"
)

func testDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.CreditBalance{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestCreditSpendAtomic(t *testing.T) {
	db := testDB(t)
	c := NewCreditService(db)
	uid := "u1"
	if err := c.Spend(uid, 1); err != nil {
		t.Fatal(err)
	}
	if c.Balance(uid) != 2 {
		t.Fatalf("balance %d", c.Balance(uid))
	}
	var wg sync.WaitGroup
	errs := make(chan error, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- c.Spend(uid, 1)
		}()
	}
	wg.Wait()
	close(errs)
	ok, fail := 0, 0
	for e := range errs {
		if e == nil {
			ok++
		} else {
			fail++
		}
	}
	if ok != 2 || fail != 18 {
		t.Fatalf("ok=%d fail=%d bal=%d", ok, fail, c.Balance(uid))
	}
	if c.Balance(uid) != 0 {
		t.Fatalf("final bal %d", c.Balance(uid))
	}
}
