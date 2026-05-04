package domain

import (
	"fmt"
	"math"
	"strings"
	"time"

	"gorm.io/gorm"
)

type Rate struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	TradingDate    time.Time `gorm:"index:idx_rate_date_code,unique;not null" json:"tradingDate"`
	Country        string    `gorm:"size:64;not null" json:"country"`
	CurrencyName   string    `gorm:"size:64;not null" json:"currencyName"`
	CurrencyCode   string    `gorm:"size:3;index:idx_rate_date_code,unique;not null" json:"currencyCode"`
	Amount         int       `gorm:"not null" json:"amount"`
	Rate           float64   `gorm:"type:numeric(12,6);not null" json:"rate"`
	NormalizedRate float64   `gorm:"type:numeric(12,6);not null" json:"normalizedRate"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

func (r *Rate) Normalize() error {
	if r == nil {
		return fmt.Errorf("rate is nil")
	}

	if r.Amount <= 0 {
		return fmt.Errorf("amount must be positive")
	}

	if r.TradingDate.IsZero() {
		return fmt.Errorf("trading date must be set")
	}

	if strings.TrimSpace(r.CurrencyCode) == "" {
		return fmt.Errorf("currency code must not be empty")
	}

	r.CurrencyCode = strings.ToUpper(strings.TrimSpace(r.CurrencyCode))
	r.TradingDate = r.TradingDate.UTC()
	r.NormalizedRate = math.Round((r.Rate/float64(r.Amount))*1_000_000) / 1_000_000

	return nil
}

func (r *Rate) BeforeSave(_ *gorm.DB) error {
	return r.Normalize()
}
