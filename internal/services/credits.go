package services

import (
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"uvo/internal/models"
)

// FreeCredits is the new-user grant (shown in pricing as «≈N песен»).
const FreeCredits = 2

type CreditService struct {
	db   *gorm.DB
	free int
}

func NewCreditService(db *gorm.DB) *CreditService {
	return &CreditService{db: db, free: FreeCredits}
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

// CreditPack is a purchasable credit bundle.
type CreditPack struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Credits    int    `json:"credits"`
	PriceRub   int    `json:"price_rub"`
	RubPerSong float64 `json:"rub_per_song"` // price/credits at 1 credit = 1 song
	Featured   bool   `json:"featured,omitempty"`
	Badge      string `json:"badge,omitempty"`
}

var CreditPacks = []CreditPack{
	{ID: "pack5", Name: "Старт · 5 песен", Credits: 5, PriceRub: 99, Featured: true, Badge: "вход"},
	{ID: "pack10", Name: "10 кредитов", Credits: 10, PriceRub: 199},
	{ID: "pack30", Name: "30 кредитов", Credits: 30, PriceRub: 499},
	{ID: "pack100", Name: "100 кредитов", Credits: 100, PriceRub: 699, Badge: "выгодно"},
	{ID: "pack500", Name: "500 кредитов", Credits: 500, PriceRub: 1690},
	{ID: "pack2000", Name: "2000 кредитов", Credits: 2000, PriceRub: 5990},
}

func init() {
	for i := range CreditPacks {
		p := &CreditPacks[i]
		if p.Credits > 0 {
			p.RubPerSong = float64(p.PriceRub) / float64(p.Credits)
		}
	}
}

// PacksPublic returns packs with computed ₽/песня for API/UI.
func PacksPublic() []CreditPack {
	out := make([]CreditPack, len(CreditPacks))
	copy(out, CreditPacks)
	return out
}

// PackByID returns credits and price for a known pack.
func PackByID(id string) (credits, priceRub int, name string, ok bool) {
	for _, p := range CreditPacks {
		if p.ID == id {
			return p.Credits, p.PriceRub, p.Name, true
		}
	}
	return 0, 0, "", false
}
