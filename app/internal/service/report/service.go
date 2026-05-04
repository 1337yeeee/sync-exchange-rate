package report

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"sync-exchange-rate/internal/repository"
)

type Service struct {
	repository repository.RateRepository
}

type Report struct {
	StartDate  time.Time        `json:"startDate"`
	EndDate    time.Time        `json:"endDate"`
	Currencies []CurrencyReport `json:"currencies"`
}

type CurrencyReport struct {
	CurrencyCode string   `json:"currencyCode"`
	MinRate      *float64 `json:"minRate"`
	MaxRate      *float64 `json:"maxRate"`
	AvgRate      *float64 `json:"avgRate"`
	Observations int      `json:"observations"`
}

func NewService(repo repository.RateRepository) (*Service, error) {
	if repo == nil {
		return nil, fmt.Errorf("rate repository must not be nil")
	}

	return &Service{repository: repo}, nil
}

func (s *Service) BuildReport(ctx context.Context, startDate, endDate time.Time, currencies []string) (Report, error) {
	if startDate.IsZero() || endDate.IsZero() {
		return Report{}, fmt.Errorf("report period must be set")
	}

	startDate = startDate.UTC()
	endDate = endDate.UTC()
	if startDate.After(endDate) {
		return Report{}, fmt.Errorf("report start date must be before or equal to end date")
	}

	normalizedCurrencies := normalizeCurrencies(currencies)
	if len(normalizedCurrencies) == 0 {
		return Report{}, fmt.Errorf("report currencies must not be empty")
	}

	rates, err := s.repository.GetByPeriod(ctx, normalizedCurrencies, startDate, endDate)
	if err != nil {
		return Report{}, err
	}

	grouped := make(map[string][]float64, len(normalizedCurrencies))
	for _, rate := range rates {
		grouped[strings.ToUpper(strings.TrimSpace(rate.CurrencyCode))] = append(
			grouped[strings.ToUpper(strings.TrimSpace(rate.CurrencyCode))],
			rate.NormalizedRate,
		)
	}

	report := Report{
		StartDate:  startDate,
		EndDate:    endDate,
		Currencies: make([]CurrencyReport, 0, len(normalizedCurrencies)),
	}

	for _, currency := range normalizedCurrencies {
		report.Currencies = append(report.Currencies, buildCurrencyReport(currency, grouped[currency]))
	}

	return report, nil
}

func buildCurrencyReport(currency string, values []float64) CurrencyReport {
	if len(values) == 0 {
		return CurrencyReport{
			CurrencyCode: currency,
			Observations: 0,
		}
	}

	minRate := values[0]
	maxRate := values[0]
	sum := 0.0

	for _, value := range values {
		if value < minRate {
			minRate = value
		}

		if value > maxRate {
			maxRate = value
		}

		sum += value
	}

	avgRate := roundToSix(sum / float64(len(values)))
	minRate = roundToSix(minRate)
	maxRate = roundToSix(maxRate)

	return CurrencyReport{
		CurrencyCode: currency,
		MinRate:      float64Ptr(minRate),
		MaxRate:      float64Ptr(maxRate),
		AvgRate:      float64Ptr(avgRate),
		Observations: len(values),
	}
}

func normalizeCurrencies(currencies []string) []string {
	result := make([]string, 0, len(currencies))
	seen := make(map[string]struct{}, len(currencies))

	for _, currency := range currencies {
		code := strings.ToUpper(strings.TrimSpace(currency))
		if code == "" {
			continue
		}

		if _, exists := seen[code]; exists {
			continue
		}

		seen[code] = struct{}{}
		result = append(result, code)
	}

	return result
}

func roundToSix(value float64) float64 {
	return math.Round(value*1_000_000) / 1_000_000
}

func float64Ptr(value float64) *float64 {
	return &value
}
