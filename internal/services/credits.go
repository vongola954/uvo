package services

import (
	"fmt"
	"os"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"uvo/internal/models"
)

// FreeCredits is the new-user grant (shown in pricing as «≈N песен»).
const FreeCredits = 2

// UnlimitedBalance is shown in UI when CREDITS_UNLIMITED=true (testing).
const UnlimitedBalance = 99999

type CreditService struct {
	db   *gorm.DB
	free int
}

func NewCreditService(db *gorm.DB) *CreditService {
	return &CreditService{db: db, free: FreeCredits}
}

// CreditsUnlimited is true when CREDITS_UNLIMITED=1|true (testing: no spend / rate limits).
func CreditsUnlimited() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("CREDITS_UNLIMITED"))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
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

func (c *CreditService) ensureTx(tx *gorm.DB, userID string) (*models.CreditBalance, error) {
	var row models.CreditBalance
	err := tx.Where(models.CreditBalance{UserID: userID}).
		Attrs(models.CreditBalance{Balance: c.free}).
		FirstOrCreate(&row).Error
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (c *CreditService) Balance(userID string) int {
	if CreditsUnlimited() {
		return UnlimitedBalance
	}
	row, err := c.ensure(userID)
	if err != nil {
		return 0
	}
	return row.Balance
}

// Spend atomically decrements balance if enough credits exist.
func (c *CreditService) Spend(userID string, n int) error {
	if CreditsUnlimited() {
		return nil
	}
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

// Add increments balance; returns error on DB failure.
func (c *CreditService) Add(userID string, n int) error {
	if n <= 0 {
		return fmt.Errorf("invalid add amount")
	}
	if _, err := c.ensure(userID); err != nil {
		return err
	}
	res := c.db.Model(&models.CreditBalance{}).
		Where("user_id = ?", userID).
		Update("balance", gorm.Expr("balance + ?", n))
	return res.Error
}

func (c *CreditService) AddTx(tx *gorm.DB, userID string, n int) error {
	if n <= 0 {
		return fmt.Errorf("invalid add amount")
	}
	if _, err := c.ensureTx(tx, userID); err != nil {
		return err
	}
	return tx.Model(&models.CreditBalance{}).
		Where("user_id = ?", userID).
		Update("balance", gorm.Expr("balance + ?", n)).Error
}

// Refund returns credits after failed provider call (best-effort).
func (c *CreditService) Refund(userID string, n int) {
	_ = c.Add(userID, n)
}

// SettlePaymentCAS marks pending order succeeded and credits user in one transaction.
// Returns (settled, error). settled=false if already processed.
func (c *CreditService) SettlePaymentCAS(orderID, providerPaymentID string, credits int) (settled bool, err error) {
	if credits <= 0 {
		return false, fmt.Errorf("invalid credits")
	}
	err = c.db.Transaction(func(tx *gorm.DB) error {
		var order models.PaymentOrder
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			First(&order, "id = ?", orderID).Error; err != nil {
			return err
		}
		if order.Status == "succeeded" {
			settled = false
			return nil
		}
		if order.Status != "pending" {
			return fmt.Errorf("order status %s", order.Status)
		}
		res := tx.Model(&models.PaymentOrder{}).
			Where("id = ? AND status = ?", orderID, "pending").
			Updates(map[string]interface{}{
				"status":              "succeeded",
				"provider_payment_id": providerPaymentID,
				"updated_at":          time.Now(),
			})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			settled = false
			return nil
		}
		if err := c.AddTx(tx, order.UserID, credits); err != nil {
			return err
		}
		settled = true
		return nil
	})
	return settled, err
}

// SpendTx locks the row inside an existing transaction (Postgres); SQLite ignores FOR UPDATE.
func (c *CreditService) SpendTx(tx *gorm.DB, userID string, n int) error {
	if CreditsUnlimited() {
		return nil
	}
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
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Credits    int     `json:"credits"`
	PriceRub   int     `json:"price_rub"`
	RubPerSong float64 `json:"rub_per_song"`
	Featured   bool    `json:"featured,omitempty"`
	Badge      string  `json:"badge,omitempty"`
}

var CreditPacks = []CreditPack{
	{ID: "pack5", Name: "Лёгкий старт", Credits: 5, PriceRub: 99, Featured: true, Badge: "ТОП"},
	{ID: "pack10", Name: "Создай хит", Credits: 10, PriceRub: 199},
	{ID: "pack30", Name: "Будь на уровне", Credits: 30, PriceRub: 499},
	{ID: "pack100", Name: "Стань звездой", Credits: 100, PriceRub: 699, Badge: "выгодно"},
	{ID: "pack500", Name: "Мастер", Credits: 500, PriceRub: 1690, Badge: "макс. выгода"},
	{ID: "pack2000", Name: "Студия", Credits: 2000, PriceRub: 5990},
}

func init() {
	for i := range CreditPacks {
		p := &CreditPacks[i]
		if p.Credits > 0 {
			p.RubPerSong = float64(p.PriceRub) / float64(p.Credits)
		}
	}
}

func PacksPublic() []CreditPack {
	out := make([]CreditPack, len(CreditPacks))
	copy(out, CreditPacks)
	return out
}

func PackByID(id string) (credits, priceRub int, name string, ok bool) {
	for _, p := range CreditPacks {
		if p.ID == id {
			return p.Credits, p.PriceRub, p.Name, true
		}
	}
	return 0, 0, "", false
}

// DualOutputEnabled reports DUAL_OUTPUT=true (AceData multi-clip keep).
func DualOutputEnabled() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv("DUAL_OUTPUT")), "true")
}

func DualPolicyLabel() string {
	if DualOutputEnabled() {
		return "on"
	}
	return "off"
}
