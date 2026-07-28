package services

import (
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"uvo/internal/models"
)

type CreditService struct {
	db   *gorm.DB
	free int
}

func NewCreditService(db *gorm.DB) *CreditService {
	return &CreditService{db: db, free: 3}
}

func (c *CreditService) ensure(userID string) (*models.CreditBalance, error) {
	var row models.CreditBalance
	err := c.db.Where(models.CreditBalance{UserID: userID}).
		Attrs(models.CreditBalance{Balance: c.free}).
		FirstOrCreate(&row).Error
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (c *CreditService) Balance(userID string) int {
	row, err := c.ensure(userID)
	if err != nil {
		return 0
	}
	return row.Balance
}

// Spend atomically decrements balance if enough credits exist.
func (c *CreditService) Spend(userID string, n int) error {
	if n <= 0 {
		return fmt.Errorf("invalid spend amount")
	}
	if _, err := c.ensure(userID); err != nil {
		return err
	}
	res := c.db.Model(&models.CreditBalance{}).
		Where("user_id = ? AND balance >= ?", userID, n).
		Update("balance", gorm.Expr("balance - ?", n))
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("недостаточно кредитов (баланс %d)", c.Balance(userID))
	}
	return nil
}

func (c *CreditService) Add(userID string, n int) {
	if n <= 0 {
		return
	}
	_, _ = c.ensure(userID)
	_ = c.db.Model(&models.CreditBalance{}).
		Where("user_id = ?", userID).
		Update("balance", gorm.Expr("balance + ?", n)).Error
}

// Refund returns credits after failed provider call.
func (c *CreditService) Refund(userID string, n int) {
	c.Add(userID, n)
}

// SpendTx locks the row inside an existing transaction (Postgres); SQLite ignores FOR UPDATE.
func (c *CreditService) SpendTx(tx *gorm.DB, userID string, n int) error {
	if n <= 0 {
		return fmt.Errorf("invalid spend amount")
	}
	var row models.CreditBalance
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where(models.CreditBalance{UserID: userID}).
		Attrs(models.CreditBalance{Balance: c.free}).
		FirstOrCreate(&row).Error
	if err != nil {
		return err
	}
	if row.Balance < n {
		return fmt.Errorf("недостаточно кредитов (баланс %d)", row.Balance)
	}
	return tx.Model(&row).Update("balance", row.Balance-n).Error
}

var CreditPacks = []map[string]interface{}{
	{"id": "pack10", "name": "10 генераций", "credits": 10, "price_rub": 199},
	{"id": "pack30", "name": "30 генераций", "credits": 30, "price_rub": 499},
	{"id": "pack100", "name": "100 генераций", "credits": 100, "price_rub": 1299},
}
