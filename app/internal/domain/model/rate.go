package model

import "time"

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
