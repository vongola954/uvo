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
	// Unique DSN per test — shared :memory: races across packages/tests.
	dsn := "file:credits_" + t.Name() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
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
	if c.Balance(uid) != FreeCredits-1 {
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
	wantOK := FreeCredits - 1
	if ok != wantOK || fail != 20-wantOK {
		t.Fatalf("ok=%d fail=%d bal=%d", ok, fail, c.Balance(uid))
	}
	if c.Balance(uid) != 0 {
		t.Fatalf("final bal %d", c.Balance(uid))
	}
}

func TestCreditRefund(t *testing.T) {
	db := testDB(t)
	c := NewCreditService(db)
	uid := "refund-u"
	_ = c.Spend(uid, 1)
	c.Refund(uid, 1)
	if c.Balance(uid) != FreeCredits {
		t.Fatalf("after refund want %d, got %d", FreeCredits, c.Balance(uid))
	}
}
