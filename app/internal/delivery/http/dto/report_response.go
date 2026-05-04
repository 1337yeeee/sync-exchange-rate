package dto

import (
	"time"

	reportservice "sync-exchange-rate/internal/service/report"
)

type ReportResponse struct {
	StartDate  string                   `json:"startDate"`
	EndDate    string                   `json:"endDate"`
	Currencies []CurrencyReportResponse `json:"currencies"`
}

type CurrencyReportResponse struct {
	CurrencyCode string   `json:"currencyCode"`
	MinRate      *float64 `json:"minRate"`
	MaxRate      *float64 `json:"maxRate"`
	AvgRate      *float64 `json:"avgRate"`
	Observations int      `json:"observations"`
}

func NewReportResponse(report reportservice.Report) ReportResponse {
	response := ReportResponse{
		StartDate:  formatDate(report.StartDate),
		EndDate:    formatDate(report.EndDate),
		Currencies: make([]CurrencyReportResponse, 0, len(report.Currencies)),
	}

	for _, currency := range report.Currencies {
		response.Currencies = append(response.Currencies, CurrencyReportResponse{
			CurrencyCode: currency.CurrencyCode,
			MinRate:      currency.MinRate,
			MaxRate:      currency.MaxRate,
			AvgRate:      currency.AvgRate,
			Observations: currency.Observations,
		})
	}

	return response
}

func formatDate(date time.Time) string {
	return date.UTC().Format(dateLayout)
}
