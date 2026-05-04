package sync

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"sync-exchange-rate/internal/domain"
)

const serviceDailyFixture = `26 Jul 2019 #143
Country|Currency|Amount|Code|Rate
EMU|euro|1|EUR|25.540
USA|dollar|1|USD|22.932
`

const serviceYearFixture2019 = `Date|1 EUR|1 USD
26.07.2019|25.540|22.932
27.07.2019|25.600|22.950
`

type fakeCNBClient struct {
	dailyResponses map[string]string
	dailyErrors    map[string]error
	yearResponses  map[int]string
	yearErrors     map[int]error
	dailyCalls     int
	yearCalls      int
}

func (c *fakeCNBClient) FetchDaily(_ context.Context, date time.Time) (string, error) {
	c.dailyCalls++
	key := date.UTC().Format("2006-01-02")
	if err, ok := c.dailyErrors[key]; ok {
		return "", err
	}

	return c.dailyResponses[key], nil
}

func (c *fakeCNBClient) FetchYear(_ context.Context, year int) (string, error) {
	c.yearCalls++
	if err, ok := c.yearErrors[year]; ok {
		return "", err
	}

	return c.yearResponses[year], nil
}

type fakeRateRepository struct {
	saveErr error
	getErr  error
	rates   map[string]domain.Rate
}

func newFakeRateRepository() *fakeRateRepository {
	return &fakeRateRepository{
		rates: make(map[string]domain.Rate),
	}
}

func (r *fakeRateRepository) Save(_ context.Context, rates []domain.Rate) error {
	if r.saveErr != nil {
		return r.saveErr
	}

	for _, rate := range rates {
		r.rates[rateKey(rate)] = rate
	}

	return nil
}

func (r *fakeRateRepository) GetByPeriod(_ context.Context, currencies []string, start, end time.Time) ([]domain.Rate, error) {
	if r.getErr != nil {
		return nil, r.getErr
	}

	filter := normalizeServiceCurrencies(currencies)
	result := make([]domain.Rate, 0, len(r.rates))
	for _, rate := range r.rates {
		if !start.IsZero() && rate.TradingDate.Before(start.UTC()) {
			continue
		}

		if !end.IsZero() && rate.TradingDate.After(end.UTC()) {
			continue
		}

		if len(filter) > 0 && !containsCurrency(filter, rate.CurrencyCode) {
			continue
		}

		result = append(result, rate)
	}

	return result, nil
}

func (r *fakeRateRepository) GetExistingDates(_ context.Context, start, end time.Time) (map[time.Time]bool, error) {
	result := make(map[time.Time]bool)
	for _, rate := range r.rates {
		if !start.IsZero() && rate.TradingDate.Before(start.UTC()) {
			continue
		}

		if !end.IsZero() && rate.TradingDate.After(end.UTC()) {
			continue
		}

		result[rate.TradingDate.UTC()] = true
	}

	return result, nil
}

func TestSyncDateSynchronizesSingleDate(t *testing.T) {
	t.Parallel()

	client := &fakeCNBClient{
		dailyResponses: map[string]string{
			"2019-07-26": serviceDailyFixture,
		},
		dailyErrors:   map[string]error{},
		yearResponses: map[int]string{},
		yearErrors:    map[int]error{},
	}
	repo := newFakeRateRepository()

	service, err := NewService(client, repo, []string{"USD", "EUR"})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	result, err := service.SyncDate(context.Background(), time.Date(2019, time.July, 26, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("SyncDate() error = %v", err)
	}

	if result.SavedCount != 2 {
		t.Fatalf("SavedCount = %d, want 2", result.SavedCount)
	}

	if result.SkippedCount != 0 {
		t.Fatalf("SkippedCount = %d, want 0", result.SkippedCount)
	}

	if len(result.Errors) != 0 {
		t.Fatalf("Errors = %v, want none", result.Errors)
	}

	if client.dailyCalls != 1 || client.yearCalls != 0 {
		t.Fatalf("dailyCalls = %d, yearCalls = %d; want 1, 0", client.dailyCalls, client.yearCalls)
	}
}

func TestSyncPeriodSynchronizesUsingYearAPI(t *testing.T) {
	t.Parallel()

	client := &fakeCNBClient{
		dailyResponses: map[string]string{},
		dailyErrors:    map[string]error{},
		yearResponses: map[int]string{
			2019: serviceYearFixture2019,
		},
		yearErrors: map[int]error{},
	}
	repo := newFakeRateRepository()

	service, err := NewService(client, repo, []string{"USD", "EUR"})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	result, err := service.SyncPeriod(
		context.Background(),
		time.Date(2019, time.July, 26, 0, 0, 0, 0, time.UTC),
		time.Date(2019, time.July, 27, 0, 0, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatalf("SyncPeriod() error = %v", err)
	}

	if result.SavedCount != 4 {
		t.Fatalf("SavedCount = %d, want 4", result.SavedCount)
	}

	if client.dailyCalls != 0 || client.yearCalls != 1 {
		t.Fatalf("dailyCalls = %d, yearCalls = %d; want 0, 1", client.dailyCalls, client.yearCalls)
	}
}

func TestSyncPeriodHandlesPartiallyMissingData(t *testing.T) {
	t.Parallel()

	client := &fakeCNBClient{
		dailyResponses: map[string]string{},
		dailyErrors:    map[string]error{},
		yearResponses: map[int]string{
			2019: serviceYearFixture2019,
		},
		yearErrors: map[int]error{
			2020: fmt.Errorf("status 404"),
		},
	}
	repo := newFakeRateRepository()

	service, err := NewService(client, repo, []string{"USD", "EUR"})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	result, err := service.SyncPeriod(
		context.Background(),
		time.Date(2019, time.July, 26, 0, 0, 0, 0, time.UTC),
		time.Date(2020, time.January, 2, 0, 0, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatalf("SyncPeriod() error = %v", err)
	}

	if result.SavedCount != 4 {
		t.Fatalf("SavedCount = %d, want 4", result.SavedCount)
	}

	if len(result.Errors) != 1 {
		t.Fatalf("len(Errors) = %d, want 1", len(result.Errors))
	}

	if !strings.Contains(result.Errors[0], "fetch year 2020") {
		t.Fatalf("Errors[0] = %q, want year 2020 fetch error", result.Errors[0])
	}
}

func TestSyncDateReturnsClientErrorInResult(t *testing.T) {
	t.Parallel()

	client := &fakeCNBClient{
		dailyResponses: map[string]string{},
		dailyErrors: map[string]error{
			"2019-07-26": fmt.Errorf("status 404"),
		},
		yearResponses: map[int]string{},
		yearErrors:    map[int]error{},
	}
	repo := newFakeRateRepository()

	service, err := NewService(client, repo, []string{"USD", "EUR"})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	result, err := service.SyncDate(context.Background(), time.Date(2019, time.July, 26, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("SyncDate() error = %v, want nil", err)
	}

	if result.SavedCount != 0 {
		t.Fatalf("SavedCount = %d, want 0", result.SavedCount)
	}

	if len(result.Errors) != 1 {
		t.Fatalf("len(Errors) = %d, want 1", len(result.Errors))
	}
}

func TestSyncDateReturnsRepositoryError(t *testing.T) {
	t.Parallel()

	client := &fakeCNBClient{
		dailyResponses: map[string]string{
			"2019-07-26": serviceDailyFixture,
		},
		dailyErrors:   map[string]error{},
		yearResponses: map[int]string{},
		yearErrors:    map[int]error{},
	}
	repo := newFakeRateRepository()
	repo.saveErr = fmt.Errorf("db unavailable")

	service, err := NewService(client, repo, []string{"USD", "EUR"})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	result, err := service.SyncDate(context.Background(), time.Date(2019, time.July, 26, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("SyncDate() error = %v, want nil", err)
	}

	if result.SavedCount != 0 {
		t.Fatalf("SavedCount = %d, want 0", result.SavedCount)
	}

	if len(result.Errors) != 1 || !strings.Contains(result.Errors[0], "save rates") {
		t.Fatalf("Errors = %v, want save error", result.Errors)
	}
}

func TestSyncDateIsIdempotent(t *testing.T) {
	t.Parallel()

	client := &fakeCNBClient{
		dailyResponses: map[string]string{
			"2019-07-26": serviceDailyFixture,
		},
		dailyErrors:   map[string]error{},
		yearResponses: map[int]string{},
		yearErrors:    map[int]error{},
	}
	repo := newFakeRateRepository()

	service, err := NewService(client, repo, []string{"USD", "EUR"})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	firstResult, err := service.SyncDate(context.Background(), time.Date(2019, time.July, 26, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("first SyncDate() error = %v", err)
	}

	secondResult, err := service.SyncDate(context.Background(), time.Date(2019, time.July, 26, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("second SyncDate() error = %v", err)
	}

	if firstResult.SavedCount != 2 {
		t.Fatalf("first SavedCount = %d, want 2", firstResult.SavedCount)
	}

	if secondResult.SavedCount != 0 {
		t.Fatalf("second SavedCount = %d, want 0", secondResult.SavedCount)
	}

	if secondResult.SkippedCount != 2 {
		t.Fatalf("second SkippedCount = %d, want 2", secondResult.SkippedCount)
	}
}

func TestSyncDateFiltersCurrenciesFromConfig(t *testing.T) {
	t.Parallel()

	client := &fakeCNBClient{
		dailyResponses: map[string]string{
			"2019-07-26": serviceDailyFixture,
		},
		dailyErrors:   map[string]error{},
		yearResponses: map[int]string{},
		yearErrors:    map[int]error{},
	}
	repo := newFakeRateRepository()

	service, err := NewService(client, repo, []string{"USD"})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	result, err := service.SyncDate(context.Background(), time.Date(2019, time.July, 26, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("SyncDate() error = %v", err)
	}

	if result.SavedCount != 1 {
		t.Fatalf("SavedCount = %d, want 1", result.SavedCount)
	}

	if len(repo.rates) != 1 {
		t.Fatalf("len(repo.rates) = %d, want 1", len(repo.rates))
	}

	for _, rate := range repo.rates {
		if rate.CurrencyCode != "USD" {
			t.Fatalf("CurrencyCode = %q, want USD", rate.CurrencyCode)
		}
	}
}

func containsCurrency(currencies []string, code string) bool {
	for _, currency := range currencies {
		if currency == strings.ToUpper(strings.TrimSpace(code)) {
			return true
		}
	}

	return false
}
