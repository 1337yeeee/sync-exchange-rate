package model

import "time"

type Report struct {
	CurrencyCode string    `json:"currencyCode"`
	StartDate    time.Time `json:"startDate"`
	EndDate      time.Time `json:"endDate"`
	MinRate      float64   `json:"minRate"`
	MaxRate      float64   `json:"maxRate"`
	AvgRate      float64   `json:"avgRate"`
	Observations int64     `json:"observations"`
}
