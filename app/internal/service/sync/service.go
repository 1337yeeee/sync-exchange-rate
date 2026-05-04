package sync

import (
	"context"
	"fmt"
	"strings"
	"time"

	"sync-exchange-rate/internal/client/cnb"
	"sync-exchange-rate/internal/domain"
	"sync-exchange-rate/internal/repository"
)

type cnbClient interface {
	FetchDaily(ctx context.Context, date time.Time) (string, error)
	FetchYear(ctx context.Context, year int) (string, error)
}

type Service struct {
	client     cnbClient
	repository repository.RateRepository
	currencies []string
}

type Result struct {
	SavedCount   int
	SkippedCount int
	Errors       []string
}

func NewService(client cnbClient, repo repository.RateRepository, currencies []string) (*Service, error) {
	if client == nil {
		return nil, fmt.Errorf("cnb client must not be nil")
	}

	if repo == nil {
		return nil, fmt.Errorf("rate repository must not be nil")
	}

	normalizedCurrencies := normalizeServiceCurrencies(currencies)
	if len(normalizedCurrencies) == 0 {
		return nil, fmt.Errorf("currencies must not be empty")
	}

	return &Service{
		client:     client,
		repository: repo,
		currencies: normalizedCurrencies,
	}, nil
}

func (s *Service) SyncDate(ctx context.Context, date time.Time) (Result, error) {
	if date.IsZero() {
		return Result{}, fmt.Errorf("sync date must be set")
	}

	existing, err := s.loadExistingRates(ctx, date.UTC(), date.UTC())
	if err != nil {
		return Result{}, err
	}

	raw, err := s.client.FetchDaily(ctx, date.UTC())
	if err != nil {
		return Result{
			Errors: []string{fmt.Sprintf("fetch daily %s: %v", date.Format("2006-01-02"), err)},
		}, nil
	}

	parsedRates, err := cnb.ParseDaily(raw, s.currencies)
	if err != nil {
		return Result{
			Errors: []string{fmt.Sprintf("parse daily %s: %v", date.Format("2006-01-02"), err)},
		}, nil
	}

	return s.persistRates(ctx, parsedRates, existing), nil
}

func (s *Service) SyncPeriod(ctx context.Context, startDate, endDate time.Time) (Result, error) {
	if startDate.IsZero() || endDate.IsZero() {
		return Result{}, fmt.Errorf("sync period must be set")
	}

	startDate = startDate.UTC()
	endDate = endDate.UTC()
	if startDate.After(endDate) {
		return Result{}, fmt.Errorf("sync start date must be before or equal to end date")
	}

	if startDate.Equal(endDate) {
		return s.SyncDate(ctx, startDate)
	}

	existing, err := s.loadExistingRates(ctx, startDate, endDate)
	if err != nil {
		return Result{}, err
	}

	result := Result{}
	for year := startDate.Year(); year <= endDate.Year(); year++ {
		raw, fetchErr := s.client.FetchYear(ctx, year)
		if fetchErr != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("fetch year %d: %v", year, fetchErr))
			continue
		}

		parsedRates, parseErr := cnb.ParseYear(raw, s.currencies, startDate, endDate)
		if parseErr != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("parse year %d: %v", year, parseErr))
			continue
		}

		yearResult := s.persistRates(ctx, parsedRates, existing)
		result.SavedCount += yearResult.SavedCount
		result.SkippedCount += yearResult.SkippedCount
		result.Errors = append(result.Errors, yearResult.Errors...)

		for _, rate := range parsedRates {
			existing[rateKey(rate)] = rate
		}
	}

	return result, nil
}

func (s *Service) loadExistingRates(ctx context.Context, startDate, endDate time.Time) (map[string]domain.Rate, error) {
	rates, err := s.repository.GetByPeriod(ctx, s.currencies, startDate, endDate)
	if err != nil {
		return nil, err
	}

	result := make(map[string]domain.Rate, len(rates))
	for _, rate := range rates {
		result[rateKey(rate)] = rate
	}

	return result, nil
}

func (s *Service) persistRates(ctx context.Context, parsedRates []domain.Rate, existing map[string]domain.Rate) Result {
	result := Result{}
	if len(parsedRates) == 0 {
		return result
	}

	toSave := make([]domain.Rate, 0, len(parsedRates))
	for _, rate := range parsedRates {
		existingRate, exists := existing[rateKey(rate)]
		if exists && sameRateData(existingRate, rate) {
			result.SkippedCount++
			continue
		}

		toSave = append(toSave, rate)
	}

	if len(toSave) == 0 {
		return result
	}

	if err := s.repository.Save(ctx, toSave); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("save rates: %v", err))
		return result
	}

	result.SavedCount = len(toSave)

	return result
}

func normalizeServiceCurrencies(currencies []string) []string {
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

func rateKey(rate domain.Rate) string {
	return rate.TradingDate.UTC().Format(time.RFC3339) + "|" + strings.ToUpper(strings.TrimSpace(rate.CurrencyCode))
}

func sameRateData(left, right domain.Rate) bool {
	return left.TradingDate.UTC().Equal(right.TradingDate.UTC()) &&
		left.Country == right.Country &&
		left.CurrencyName == right.CurrencyName &&
		strings.EqualFold(left.CurrencyCode, right.CurrencyCode) &&
		left.Amount == right.Amount &&
		left.Rate == right.Rate &&
		left.NormalizedRate == right.NormalizedRate
}
